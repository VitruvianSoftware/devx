package k3s

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// GetVersion returns the installed K3s version string.
func (m *Manager) GetVersion(ctx context.Context) (string, error) {
	out, err := m.runner.LimaShellSudo(ctx, m.vmName, "k3s --version 2>/dev/null | head -1")
	if err != nil {
		return "", err
	}
	// Output format: "k3s version v1.31.2+k3s1 (abc123)"
	parts := strings.Fields(out)
	if len(parts) >= 3 {
		return parts[2], nil
	}
	return strings.TrimSpace(out), nil
}

// ReinstallServer reinstalls K3s server (init node) with a new version.
func (m *Manager) ReinstallServer(ctx context.Context, nodeIP, version string, tlsSANs []string, pool string) error {
	var sanFlags string
	for _, san := range tlsSANs {
		sanFlags += fmt.Sprintf(" --tls-san=%s", san)
	}

	script := fmt.Sprintf(
		`curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q INSTALL_K3S_EXEC="server" sh -s - --node-ip=%s --advertise-address=%s%s --flannel-iface=lima0 --node-label=pool=%s`,
		version, nodeIP, nodeIP, sanFlags, pool,
	)

	slog.Debug("K3s reinstall server script", "host", m.runner.Host)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName, script)
	return err
}

// ReinstallJoinServer reinstalls K3s on a server node that joins an existing cluster.
func (m *Manager) ReinstallJoinServer(ctx context.Context, nodeIP, serverURL, token, version, pool string) error {
	script := fmt.Sprintf(
		`curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q K3S_TOKEN=%q INSTALL_K3S_EXEC="server" sh -s - --server=%s --node-ip=%s --advertise-address=%s --tls-san=%s --flannel-iface=lima0 --node-label=pool=%s`,
		version, token, serverURL, nodeIP, nodeIP, nodeIP, pool,
	)

	slog.Debug("K3s reinstall join-server script", "host", m.runner.Host)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName, script)
	return err
}

// ReinstallAgent reinstalls K3s on an agent node.
func (m *Manager) ReinstallAgent(ctx context.Context, nodeIP, serverURL, token, version, pool string) error {
	script := fmt.Sprintf(
		`curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q K3S_TOKEN=%q K3S_URL=%q sh -s - agent --node-ip=%s --flannel-iface=lima0 --node-label=pool=%s`,
		version, token, serverURL, nodeIP, pool,
	)

	slog.Debug("K3s reinstall agent script", "host", m.runner.Host)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName, script)
	return err
}

// UncordonNode marks a node as schedulable after draining.
func (m *Manager) UncordonNode(ctx context.Context, nodeName string) error {
	slog.Info("uncordoning node", "host", m.runner.Host, "node", nodeName)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName,
		fmt.Sprintf("k3s kubectl uncordon %s", nodeName))
	return err
}

// CreateSnapshot creates an etcd snapshot and returns the snapshot name/path.
func (m *Manager) CreateSnapshot(ctx context.Context) (string, error) {
	timestamp := time.Now().UTC().Format("20060102-150405")
	snapshotName := fmt.Sprintf("homelab-snapshot-%s", timestamp)

	slog.Info("creating etcd snapshot", "host", m.runner.Host, "name", snapshotName)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName,
		fmt.Sprintf("k3s etcd-snapshot save --name %s", snapshotName))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("/var/lib/rancher/k3s/server/db/snapshots/%s", snapshotName), nil
}

// DownloadSnapshot copies a snapshot from the remote VM to the local machine.
func (m *Manager) DownloadSnapshot(ctx context.Context, remotePath, localPath string) error {
	slog.Info("downloading snapshot", "host", m.runner.Host, "remote", remotePath, "local", localPath)

	// Read snapshot content from VM.
	content, err := m.runner.LimaShellSudo(ctx, m.vmName,
		fmt.Sprintf("base64 %s", remotePath))
	if err != nil {
		return fmt.Errorf("reading snapshot: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(content))
	if err != nil {
		return fmt.Errorf("decoding snapshot: %w", err)
	}

	if err := os.WriteFile(localPath, decoded, 0o600); err != nil {
		return fmt.Errorf("writing snapshot: %w", err)
	}

	return nil
}

// UploadSnapshot copies a snapshot from the local machine to the remote VM.
func (m *Manager) UploadSnapshot(ctx context.Context, localPath, remotePath string) error {
	slog.Info("uploading snapshot", "host", m.runner.Host, "local", localPath, "remote", remotePath)

	content, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("reading local snapshot: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(content)

	_, err = m.runner.LimaShellSudo(ctx, m.vmName,
		fmt.Sprintf("echo %s | base64 -d > %s", encoded, remotePath))
	return err
}

// RestoreSnapshot restores etcd from a snapshot file on the remote VM.
func (m *Manager) RestoreSnapshot(ctx context.Context, snapshotPath string) error {
	slog.Info("restoring etcd from snapshot", "host", m.runner.Host, "snapshot", snapshotPath)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName,
		fmt.Sprintf("k3s server --cluster-reset --cluster-reset-restore-path=%s", snapshotPath))
	return err
}

// StopService stops the K3s systemd service.
func (m *Manager) StopService(ctx context.Context) error {
	slog.Info("stopping K3s service", "host", m.runner.Host)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName, "systemctl stop k3s")
	return err
}

// StartService starts the K3s systemd service.
func (m *Manager) StartService(ctx context.Context) error {
	slog.Info("starting K3s service", "host", m.runner.Host)
	_, err := m.runner.LimaShellSudo(ctx, m.vmName, "systemctl start k3s")
	return err
}
