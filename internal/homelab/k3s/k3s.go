// Package k3s handles K3s installation, cluster join, and lifecycle management
// on remote Lima VMs.
package k3s

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/homelab/remote"
)

const defaultVMName = "k8s-node"

// Manager handles K3s operations on a remote Lima VM.
type Manager struct {
	runner *remote.Runner
	vmName string
}

// NewManager creates a new K3s manager for the given remote host.
func NewManager(runner *remote.Runner) *Manager {
	return &Manager{runner: runner, vmName: defaultVMName}
}

// NewManagerWithVM creates a new K3s manager with a custom VM name.
func NewManagerWithVM(runner *remote.Runner, vmName string) *Manager {
	if vmName == "" {
		vmName = defaultVMName
	}
	return &Manager{runner: runner, vmName: vmName}
}

// IsInstalled checks whether K3s is installed inside the VM.
func (m *Manager) IsInstalled(ctx context.Context) (bool, error) {
	out, err := m.runner.LimaShellSudo(ctx, m.vmName, "test -f /usr/local/bin/k3s && echo yes || echo no")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "yes", nil
}

// InitCluster bootstraps the first control plane node with --cluster-init.
func (m *Manager) InitCluster(ctx context.Context, nodeIP, pool, k3sVersion string, tlsSANs []string) error {
	installed, err := m.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		slog.Debug("K3s already installed, skipping init", "host", m.runner.Host)
		fmt.Printf("  [%s] K3s already installed, skipping init\n", m.runner.Host)
		return nil
	}

	slog.Info("initializing K3s cluster", "host", m.runner.Host, "node_ip", nodeIP, "pool", pool)
	fmt.Printf("  [%s] Initializing K3s cluster...\n", m.runner.Host)

	var sanFlags string
	for _, san := range tlsSANs {
		sanFlags += fmt.Sprintf(" --tls-san=%s", san)
	}

	versionEnv := ""
	if k3sVersion != "" {
		versionEnv = fmt.Sprintf("INSTALL_K3S_VERSION=%q ", k3sVersion)
	}

	script := fmt.Sprintf(
		`curl -sfL https://get.k3s.io | %sINSTALL_K3S_EXEC="server" sh -s - --cluster-init --node-name=%s --node-ip=%s --advertise-address=%s%s --flannel-iface=lima0 --node-label=pool=%s`,
		versionEnv, m.runner.Host, nodeIP, nodeIP, sanFlags, pool,
	)

	slog.Debug("K3s install script", "host", m.runner.Host, "script", script)
	_, err = m.runner.LimaShellSudo(ctx, m.vmName, script)
	return err
}

// JoinServer joins a server node to an existing HA cluster.
func (m *Manager) JoinServer(ctx context.Context, nodeIP, serverURL, token, pool, k3sVersion string, tlsSANs []string) error {
	installed, err := m.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		slog.Debug("K3s already installed, skipping join", "host", m.runner.Host)
		fmt.Printf("  [%s] K3s already installed, skipping join\n", m.runner.Host)
		return nil
	}

	slog.Info("joining cluster as server", "host", m.runner.Host, "node_ip", nodeIP, "server_url", serverURL)
	fmt.Printf("  [%s] Joining cluster as server...\n", m.runner.Host)

	var sanFlags string
	for _, san := range tlsSANs {
		sanFlags += fmt.Sprintf(" --tls-san=%s", san)
	}

	versionEnv := ""
	if k3sVersion != "" {
		versionEnv = fmt.Sprintf("INSTALL_K3S_VERSION=%q ", k3sVersion)
	}

	// Install K3s binary and create systemd service without starting it,
	// because the join may need multiple retries as etcd stabilizes.
	script := fmt.Sprintf(
		`curl -sfL https://get.k3s.io | %sINSTALL_K3S_SKIP_START=true K3S_TOKEN=%q INSTALL_K3S_EXEC="server" sh -s - --server=%s --node-name=%s --node-ip=%s --advertise-address=%s%s --flannel-iface=lima0 --node-label=pool=%s`,
		versionEnv, token, serverURL, m.runner.Host, nodeIP, nodeIP, sanFlags, pool,
	)

	slog.Debug("K3s join script", "host", m.runner.Host)
	_, err = m.runner.LimaShellSudo(ctx, m.vmName, script)
	if err != nil {
		return err
	}

	// Now start K3s and let it retry joining on its own.
	fmt.Printf("  [%s] Starting K3s server (joining cluster)...\n", m.runner.Host)
	_, err = m.runner.LimaShellSudo(ctx, m.vmName, "systemctl start k3s --no-block")
	if err != nil {
		return fmt.Errorf("[%s] starting k3s service: %w", m.runner.Host, err)
	}

	// Wait for K3s to join successfully by checking the API.
	return m.waitForJoin(ctx, 3*time.Minute)
}

// waitForJoin waits until the K3s service is running and has joined the cluster.
func (m *Manager) waitForJoin(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return fmt.Errorf("[%s] context cancelled while waiting for K3s join: %w", m.runner.Host, ctx.Err())
		}

		// Check if K3s process is running (active).
		out, err := m.runner.LimaShellSudo(ctx, m.vmName, "systemctl is-active k3s 2>/dev/null")
		if err == nil && strings.TrimSpace(out) == "active" {
			slog.Info("K3s joined successfully", "host", m.runner.Host)
			return nil
		}

		slog.Debug("waiting for K3s to join cluster", "host", m.runner.Host)
		fmt.Printf("  [%s] Waiting for K3s to join cluster...\n", m.runner.Host)
		select {
		case <-ctx.Done():
			return fmt.Errorf("[%s] context cancelled: %w", m.runner.Host, ctx.Err())
		case <-time.After(10 * time.Second):
		}
	}

	return fmt.Errorf("[%s] timed out waiting for K3s to join cluster", m.runner.Host)
}

// JoinAgent joins a worker node to the cluster.
func (m *Manager) JoinAgent(ctx context.Context, nodeIP, serverURL, token, pool, k3sVersion string) error {
	installed, err := m.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if installed {
		slog.Debug("K3s already installed, skipping join", "host", m.runner.Host)
		fmt.Printf("  [%s] K3s already installed, skipping join\n", m.runner.Host)
		return nil
	}

	slog.Info("joining cluster as agent", "host", m.runner.Host, "node_ip", nodeIP, "server_url", serverURL)
	fmt.Printf("  [%s] Joining cluster as agent...\n", m.runner.Host)

	versionEnv := ""
	if k3sVersion != "" {
		versionEnv = fmt.Sprintf("INSTALL_K3S_VERSION=%q ", k3sVersion)
	}

	script := fmt.Sprintf(
		`curl -sfL https://get.k3s.io | %sK3S_TOKEN=%q K3S_URL=%q sh -s - agent --node-name=%s --node-ip=%s --flannel-iface=lima0 --node-label=pool=%s`,
		versionEnv, token, serverURL, m.runner.Host, nodeIP, pool,
	)

	slog.Debug("K3s agent join script", "host", m.runner.Host)
	_, err = m.runner.LimaShellSudo(ctx, m.vmName, script)
	return err
}

// GetToken retrieves the K3s node token from the server.
func (m *Manager) GetToken(ctx context.Context) (string, error) {
	out, err := m.runner.LimaShellSudo(ctx, m.vmName, "cat /var/lib/rancher/k3s/server/node-token")
	if err != nil {
		return "", fmt.Errorf("[%s] reading node token: %w", m.runner.Host, err)
	}
	return strings.TrimSpace(out), nil
}

// WaitForReady polls until K3s is initialized, the node token exists, and
// the API server is responding to kubectl commands.
func (m *Manager) WaitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check context cancellation.
		if ctx.Err() != nil {
			return fmt.Errorf("[%s] context cancelled while waiting for K3s: %w", m.runner.Host, ctx.Err())
		}

		// Check that the node-token file exists.
		out, err := m.runner.LimaShellSudo(ctx, m.vmName, "test -f /var/lib/rancher/k3s/server/node-token && echo ready || echo waiting")
		if err != nil || strings.TrimSpace(out) != "ready" {
			slog.Debug("waiting for K3s node-token", "host", m.runner.Host)
			fmt.Printf("  [%s] Waiting for K3s to initialize...\n", m.runner.Host)
			select {
			case <-ctx.Done():
				return fmt.Errorf("[%s] context cancelled while waiting for K3s: %w", m.runner.Host, ctx.Err())
			case <-time.After(5 * time.Second):
			}
			continue
		}

		// Also verify the API server is accepting connections.
		_, err = m.runner.LimaShellSudo(ctx, m.vmName, "k3s kubectl get nodes --request-timeout=5s 2>/dev/null")
		if err == nil {
			slog.Info("K3s is ready", "host", m.runner.Host)
			return nil
		}

		slog.Debug("K3s token exists but API server not ready yet", "host", m.runner.Host)
		fmt.Printf("  [%s] K3s initializing (API server starting)...\n", m.runner.Host)
		select {
		case <-ctx.Done():
			return fmt.Errorf("[%s] context cancelled while waiting for K3s: %w", m.runner.Host, ctx.Err())
		case <-time.After(5 * time.Second):
		}
	}

	return fmt.Errorf("[%s] timed out waiting for K3s to become ready", m.runner.Host)
}

// GetNodeStatus returns the output of kubectl get nodes.
func (m *Manager) GetNodeStatus(ctx context.Context) (string, error) {
	return m.runner.LimaShellSudo(ctx, m.vmName, "k3s kubectl get nodes -o wide -L pool -L kubernetes.io/arch")
}

// Uninstall removes K3s from the VM.
func (m *Manager) Uninstall(ctx context.Context, role string) error {
	installed, err := m.IsInstalled(ctx)
	if err != nil {
		return err
	}
	if !installed {
		slog.Debug("K3s not installed, skipping uninstall", "host", m.runner.Host)
		return nil
	}

	slog.Info("uninstalling K3s", "host", m.runner.Host, "role", role)
	fmt.Printf("  [%s] Uninstalling K3s...\n", m.runner.Host)

	script := "/usr/local/bin/k3s-uninstall.sh"
	if role == "agent" {
		script = "/usr/local/bin/k3s-agent-uninstall.sh"
	}

	_, err = m.runner.LimaShellSudo(ctx, m.vmName, script)
	return err
}

// DrainNode drains a node before removal.
func (m *Manager) DrainNode(ctx context.Context, nodeName string) error {
	slog.Info("draining node", "host", m.runner.Host, "node", nodeName)
	fmt.Printf("  [%s] Draining node %s...\n", m.runner.Host, nodeName)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName,
		fmt.Sprintf("k3s kubectl drain %s --ignore-daemonsets --delete-emptydir-data --timeout=60s", nodeName))
	return err
}

// DeleteNode removes a node from the cluster.
func (m *Manager) DeleteNode(ctx context.Context, nodeName string) error {
	slog.Info("deleting node from cluster", "host", m.runner.Host, "node", nodeName)
	fmt.Printf("  [%s] Deleting node %s from cluster...\n", m.runner.Host, nodeName)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName,
		fmt.Sprintf("k3s kubectl delete node %s", nodeName))
	return err
}

// GetKubeconfig retrieves and patches the kubeconfig for external access.
func (m *Manager) GetKubeconfig(ctx context.Context, serverIP string) (string, error) {
	out, err := m.runner.LimaShellSudo(ctx, m.vmName, "cat /etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return "", err
	}
	// Replace localhost with the actual server IP.
	return strings.ReplaceAll(out, "127.0.0.1", serverIP), nil
}
