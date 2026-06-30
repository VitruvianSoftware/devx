// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package k3s handles K3s installation, cluster join, and lifecycle management
// on remote Lima VMs.
package k3s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
)

const defaultVMName = "k8s-node"

// execFunc runs a privileged command on the node and returns combined output.
// Lima VMs use limactl-shell-sudo; native Linux hosts use ssh-sudo.
type execFunc func(ctx context.Context, command string) (string, error)

// Manager handles K3s operations on a node, running privileged commands via exec.
// The K3s install/join recipe is identical for a Lima VM or a bare host; only how
// the command is delivered differs.
type Manager struct {
	runner *remote.Runner
	vmName string
	exec   execFunc
}

// NewManager creates a new K3s manager for the given remote host (Lima VM).
func NewManager(runner *remote.Runner) *Manager {
	return NewManagerWithVM(runner, defaultVMName)
}

// NewManagerWithVM creates a new K3s manager that runs commands inside a Lima VM.
func NewManagerWithVM(runner *remote.Runner, vmName string) *Manager {
	if vmName == "" {
		vmName = defaultVMName
	}
	return &Manager{
		runner: runner,
		vmName: vmName,
		exec: func(ctx context.Context, cmd string) (string, error) {
			return runner.LimaShellSudo(ctx, vmName, cmd)
		},
	}
}

// NewManagerWithExec creates a Manager that runs commands via the injected exec
// (e.g. ssh-sudo directly on a native Linux host). runner is used only for its
// Host label in log lines.
func NewManagerWithExec(runner *remote.Runner, exec execFunc) *Manager {
	return &Manager{runner: runner, vmName: runner.Host, exec: exec}
}

// sudo runs a privileged command on the node via the configured exec.
func (m *Manager) sudo(ctx context.Context, cmd string) (string, error) {
	return m.exec(ctx, cmd)
}

// IsInstalled checks whether K3s is installed inside the VM.
func (m *Manager) IsInstalled(ctx context.Context) (bool, error) {
	out, err := m.sudo(ctx, "test -f /usr/local/bin/k3s && echo yes || echo no")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "yes", nil
}

// InstallTailscale installs Tailscale, brings it up with the provided auth key, and returns the Tailscale IP.
func (m *Manager) InstallTailscale(ctx context.Context, authKey string) (string, error) {
	slog.Info("installing Tailscale", "host", m.runner.Host)
	fmt.Printf("  [%s] Installing and configuring Tailscale...\n", m.runner.Host)

	// Install Tailscale
	script := "curl -fsSL https://tailscale.com/install.sh | sh"
	if _, err := m.sudo(ctx, script); err != nil {
		return "", fmt.Errorf("[%s] installing tailscale: %w", m.runner.Host, err)
	}

	// Bring up Tailscale
	upCmd := fmt.Sprintf("tailscale up --authkey=%s --accept-routes", authKey)
	if _, err := m.sudo(ctx, upCmd); err != nil {
		return "", fmt.Errorf("[%s] starting tailscale: %w", m.runner.Host, err)
	}

	// Enforce --accept-routes idempotently. `tailscale up` only applies it on a
	// fresh bring-up; a node already up (joined earlier without accept-routes)
	// keeps RouteAll=false, which breaks Cilium native routing over tailscale
	// (the Fedora/nuc9 nodes). `tailscale set` re-asserts it regardless of state.
	if _, err := m.sudo(ctx, "tailscale set --accept-routes=true"); err != nil {
		return "", fmt.Errorf("[%s] enforcing tailscale accept-routes: %w", m.runner.Host, err)
	}

	// Get the Tailscale IPv4 address
	ip, err := m.sudo(ctx, "tailscale ip -4")
	if err != nil {
		return "", fmt.Errorf("[%s] getting tailscale ip: %w", m.runner.Host, err)
	}

	return strings.TrimSpace(ip), nil
}

// InitCluster bootstraps the first control plane node with --cluster-init.
func (m *Manager) InitCluster(ctx context.Context, nodeIP, pool, k3sVersion string, tlsSANs []string, disableServiceLB, useTailscale, useDocker, useCilium bool) error {
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

	script := buildServerInitScript(m.runner.Host, nodeIP, pool, k3sVersion, tlsSANs, disableServiceLB, useTailscale, useDocker, useCilium)

	slog.Debug("K3s install script", "host", m.runner.Host, "script", script)
	_, err = m.sudo(ctx, script)
	return err
}

// DeployMetalLB installs MetalLB and configures the IPAddressPool and
// L2Advertisement. When l2Hostnames is non-empty, the L2Advertisement is pinned
// to those nodes via spec.nodeSelectors (one matchLabels per hostname), so the
// speaker VIP stays on stable nodes instead of flapping onto high-latency Macs.
// An empty list emits the L2Advertisement with no nodeSelector (default MetalLB
// behaviour — advertise from all nodes).
func (m *Manager) DeployMetalLB(ctx context.Context, ipRange string, l2Hostnames []string) error {
	slog.Info("deploying metallb", "host", m.runner.Host, "ip_range", ipRange, "l2_hostnames", l2Hostnames)
	fmt.Printf("  [%s] Applying MetalLB manifests...\n", m.runner.Host)

	// Install MetalLB
	_, err := m.sudo(ctx, "k3s kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.9/config/manifests/metallb-native.yaml")
	if err != nil {
		return fmt.Errorf("applying metallb manifests: %w", err)
	}

	// HA: the upstream manifest ships the controller as a single replica with no
	// anti-affinity, making it a single point of failure for LoadBalancer IP
	// allocation (existing VIPs keep working via the speaker DaemonSet, but new or
	// changed Services stall while it is down). Patch it to 2 replicas with soft
	// anti-affinity (see metallbControllerHAPatch). Idempotent; re-applied every run.
	// NOTE: MetalLB is legacy — dev-local moved LoadBalancer IPAM/L2 to Cilium
	// LB-IPAM (GitOps); this path only runs when metallb.enabled && !cilium.enabled.
	fmt.Printf("  [%s] Patching MetalLB controller for HA (2 replicas)...\n", m.runner.Host)
	_, err = m.sudo(ctx, fmt.Sprintf("k3s kubectl -n metallb-system patch deployment controller --type=merge -p '%s'", metallbControllerHAPatch))
	if err != nil {
		slog.Warn("failed to patch metallb controller for HA, continuing with single replica", "error", err)
	}

	fmt.Printf("  [%s] Waiting for MetalLB controller to be ready...\n", m.runner.Host)
	// Give it a moment to create the pods
	time.Sleep(5 * time.Second)
	_, err = m.sudo(ctx, "k3s kubectl wait --namespace metallb-system --for=condition=ready pod --selector=component=controller --timeout=90s")
	if err != nil {
		slog.Warn("timeout waiting for metallb pods, continuing anyway", "error", err)
	}

	fmt.Printf("  [%s] Configuring MetalLB IP pool (%s)...\n", m.runner.Host, ipRange)
	configManifest := fmt.Sprintf(`apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default-pool
  namespace: metallb-system
spec:
  addresses:
  - %s
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: default-l2
  namespace: metallb-system
%s`, ipRange, l2AdvertisementNodeSelectors(l2Hostnames))

	// Write the manifest to a temp file in the VM and apply it
	encoded := base64.StdEncoding.EncodeToString([]byte(configManifest))
	cmd := fmt.Sprintf("echo '%s' | base64 -d | k3s kubectl apply -f -", encoded)
	_, err = m.sudo(ctx, cmd)
	if err != nil {
		return fmt.Errorf("applying metallb config: %w", err)
	}

	return nil
}

// l2AdvertisementNodeSelectors renders the spec.nodeSelectors block (one
// matchLabels: {kubernetes.io/hostname: <host>} entry per hostname) for the
// L2Advertisement, or the empty string when no hostnames are given (so the
// advertisement carries no spec and matches all nodes).
func l2AdvertisementNodeSelectors(hostnames []string) string {
	if len(hostnames) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("spec:\n  nodeSelectors:\n")
	for _, h := range hostnames {
		fmt.Fprintf(&b, "  - matchLabels:\n      kubernetes.io/hostname: %s\n", h)
	}
	return b.String()
}

// corednsHAPatch is a JSON merge patch that makes k3s's bundled CoreDNS
// survive a single node loss: two replicas, kept on distinct hosts by a
// *required* pod-anti-affinity on the kube-dns pods. k3s ships a single
// replica, so a DNS-hosting node going down is a cluster-wide outage.
const corednsHAPatch = `{"spec":{"replicas":2,"template":{"spec":{"affinity":{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"k8s-app","operator":"In","values":["kube-dns"]}]},"topologyKey":"kubernetes.io/hostname"}]}}}}}}`

// metallbControllerHAPatch is a JSON merge patch that makes MetalLB's controller
// survive a single node loss: 2 leader-elected replicas (one active, one hot
// standby) kept off the same host by a *soft* hostname pod-anti-affinity. The
// upstream metallb-native.yaml ships a single replica with no anti-affinity, so
// the node hosting it going down stalls all new/changed LoadBalancer IP
// allocation (existing VIPs keep serving via the speaker DaemonSet). Soft (not
// required) so a single-node cluster can still schedule both replicas.
const metallbControllerHAPatch = `{"spec":{"replicas":2,"template":{"spec":{"affinity":{"podAntiAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"weight":100,"podAffinityTerm":{"topologyKey":"kubernetes.io/hostname","labelSelector":{"matchLabels":{"component":"controller"}}}}]}}}}}}`

// defaultLocalPathDir is the k3s built-in local-path provisioner's default node
// storage path (used for any node without an explicit nodePathMap override).
const defaultLocalPathDir = "/var/lib/rancher/k3s/storage"

// buildLocalPathConfigJSON renders the local-path-config ConfigMap's config.json
// nodePathMap. A node with a custom dataDir gets <dataDir>/storage, so its
// local-path PVs land on the same large volume as the rest of k3s's data — the
// provisioner's path is independent of k3s --data-dir, so it must be steered
// separately. Every other node falls back to defaultLocalPathDir. Deterministic
// (sorted) so the rendered config is stable across runs.
func buildLocalPathConfigJSON(dataDirs map[string]string) string {
	type npEntry struct {
		Node  string   `json:"node"`
		Paths []string `json:"paths"`
	}
	entries := []npEntry{{Node: "DEFAULT_PATH_FOR_NON_LISTED_NODES", Paths: []string{defaultLocalPathDir}}}
	hosts := make([]string, 0, len(dataDirs))
	for h := range dataDirs {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	for _, h := range hosts {
		entries = append(entries, npEntry{Node: h, Paths: []string{strings.TrimRight(dataDirs[h], "/") + "/storage"}})
	}
	b, _ := json.MarshalIndent(map[string]any{"nodePathMap": entries}, "", "  ")
	return string(b)
}

// parseNodePathMap extracts node→paths from a local-path config.json as a map, for
// order-independent comparison.
func parseNodePathMap(s string) (map[string][]string, bool) {
	var c struct {
		NodePathMap []struct {
			Node  string   `json:"node"`
			Paths []string `json:"paths"`
		} `json:"nodePathMap"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(s)), &c) != nil {
		return nil, false
	}
	m := make(map[string][]string, len(c.NodePathMap))
	for _, e := range c.NodePathMap {
		m[e.Node] = e.Paths
	}
	return m, true
}

// localPathConfigEqual reports whether two config.json strings define the same
// nodePathMap (ignoring key order / formatting), so a converged cluster skips the
// patch + provisioner restart.
func localPathConfigEqual(a, b string) bool {
	ma, oka := parseNodePathMap(a)
	mb, okb := parseNodePathMap(b)
	return oka && okb && reflect.DeepEqual(ma, mb)
}

// ensureLocalPathNodePaths steers the k3s built-in local-path provisioner to a
// per-node path for each node that set a custom dataDir, so that node's local-path
// PVs land on its big volume (<dataDir>/storage) instead of the default /
// partition. The provisioner reads its path from the local-path-config ConfigMap,
// NOT from k3s --data-dir, so this is the second half of a node's dataDir
// relocation. No-op when no node sets dataDir; idempotent (patches + restarts the
// provisioner only when the live nodePathMap differs from the desired one).
func (m *Manager) ensureLocalPathNodePaths(ctx context.Context, dataDirs map[string]string) error {
	if len(dataDirs) == 0 {
		return nil
	}
	desired := buildLocalPathConfigJSON(dataDirs)
	if cur, err := m.sudo(ctx, `k3s kubectl -n kube-system get configmap local-path-config -o jsonpath='{.data.config\.json}'`); err == nil && localPathConfigEqual(cur, desired) {
		return nil // already converged
	}
	fmt.Printf("  [%s] Steering local-path provisioner to dataDir paths (%d node(s))...\n", m.runner.Host, len(dataDirs))
	patch, err := json.Marshal(map[string]any{"data": map[string]string{"config.json": desired}})
	if err != nil {
		return fmt.Errorf("building local-path-config patch: %w", err)
	}
	enc := base64.StdEncoding.EncodeToString(patch)
	patchCmd := fmt.Sprintf(
		"echo '%s' | base64 -d | k3s kubectl -n kube-system patch configmap local-path-config --type=merge --patch-file=/dev/stdin",
		enc,
	)
	if _, err := m.sudo(ctx, patchCmd); err != nil {
		return fmt.Errorf("patching local-path-config nodePathMap: %w", err)
	}
	// The provisioner caches config at startup; restart so new PVCs use the new path.
	if _, err := m.sudo(ctx, "k3s kubectl -n kube-system rollout restart deployment local-path-provisioner"); err != nil {
		return fmt.Errorf("restarting local-path-provisioner: %w", err)
	}
	return nil
}

// EnsureClusterDefaults bakes the homelab resilience defaults into a running
// cluster so they are reproducible and re-asserted on every `cluster init` /
// `cluster apply`, instead of living as one-off live edits:
//
//   - CoreDNS → 2 replicas + required hostname anti-affinity (see corednsHAPatch).
//   - Exactly one default StorageClass: k3s marks its bundled local-path as
//     default, but Longhorn (installed by the Pulumi dev-local stack) is the real
//     default. Two defaults is invalid, so local-path is un-defaulted here.
//   - local-path nodePathMap: nodes with a custom dataDir get <dataDir>/storage,
//     so their PVs follow k3s onto the big volume (see ensureLocalPathNodePaths).
//
// It is idempotent and safe to re-run: the CoreDNS patch is a no-op once applied,
// and the StorageClass step is guarded so it does nothing if local-path is absent
// (it never deletes local-path — workloads may still request it explicitly).
//
// Note on durability: a plain k3s restart preserves these (k3s only re-applies a
// bundled addon when its on-disk manifest changes), but a k3s *version upgrade*
// re-extracts the bundled manifests and can reset them — re-running this (which a
// `cluster apply` does) re-converges.
func (m *Manager) EnsureClusterDefaults(ctx context.Context, dataDirs map[string]string) error {
	slog.Info("ensuring cluster resilience defaults", "host", m.runner.Host)

	// CoreDNS HA. Deliver the patch via base64 + --patch-file to avoid the
	// nested-quoting hazards of inlining JSON through `sudo sh -c %q`.
	fmt.Printf("  [%s] Ensuring CoreDNS HA (2 replicas + anti-affinity)...\n", m.runner.Host)
	enc := base64.StdEncoding.EncodeToString([]byte(corednsHAPatch))
	corednsCmd := fmt.Sprintf(
		"echo '%s' | base64 -d | k3s kubectl -n kube-system patch deployment coredns --type=merge --patch-file=/dev/stdin",
		enc,
	)
	if _, err := m.sudo(ctx, corednsCmd); err != nil {
		return fmt.Errorf("patching coredns for HA: %w", err)
	}

	// Single default StorageClass: un-default local-path if present. Guarded so a
	// cluster without local-path (e.g. --disable local-storage) is a clean no-op.
	fmt.Printf("  [%s] Ensuring single default StorageClass (un-defaulting local-path)...\n", m.runner.Host)
	scCmd := "if k3s kubectl get storageclass local-path >/dev/null 2>&1; then " +
		"k3s kubectl annotate --overwrite storageclass local-path storageclass.kubernetes.io/is-default-class=false; fi"
	if _, err := m.sudo(ctx, scCmd); err != nil {
		return fmt.Errorf("un-defaulting local-path storageclass: %w", err)
	}

	// Steer local-path PVs onto each dataDir node's big volume (the provisioner's
	// path is independent of k3s --data-dir, so this is the second half of a node's
	// dataDir relocation). No-op when no node sets dataDir.
	if err := m.ensureLocalPathNodePaths(ctx, dataDirs); err != nil {
		return err
	}

	return nil
}

// JoinServer joins a server node to an existing HA cluster.
func (m *Manager) JoinServer(ctx context.Context, nodeIP, serverURL, token, pool, k3sVersion string, tlsSANs []string, disableServiceLB, useTailscale, useDocker, useCilium bool) error {
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

	// Install the K3s binary + systemd service without starting it, because the
	// join may need multiple retries as etcd stabilizes.
	script := buildServerJoinScript(m.runner.Host, nodeIP, serverURL, token, pool, k3sVersion, tlsSANs, disableServiceLB, useTailscale, useDocker, useCilium)

	slog.Debug("K3s join script", "host", m.runner.Host)
	_, err = m.sudo(ctx, script)
	if err != nil {
		return err
	}

	// Now start K3s and let it retry joining on its own.
	fmt.Printf("  [%s] Starting K3s server (joining cluster)...\n", m.runner.Host)
	_, err = m.sudo(ctx, "systemctl start k3s --no-block")
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
		out, err := m.sudo(ctx, "systemctl is-active k3s 2>/dev/null")
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
func (m *Manager) JoinAgent(ctx context.Context, nodeIP, serverURL, token, pool, k3sVersion string, useTailscale, useDocker, useCilium bool) error {
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

	// A k3s AGENT takes no Cilium disable flags — --flannel-backend=none,
	// --disable-kube-proxy and --disable-network-policy are SERVER-only (the agent
	// CLI rejects them); agents inherit the CNI config from the servers.
	_ = useCilium

	script := buildAgentJoinScript(m.runner.Host, nodeIP, serverURL, token, pool, k3sVersion, useTailscale, useDocker)

	slog.Debug("K3s agent join script", "host", m.runner.Host)
	_, err = m.sudo(ctx, script)
	return err
}

// GetToken retrieves the K3s node token from the server.
func (m *Manager) GetToken(ctx context.Context) (string, error) {
	out, err := m.sudo(ctx, "cat /var/lib/rancher/k3s/server/node-token")
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
		out, err := m.sudo(ctx, "test -f /var/lib/rancher/k3s/server/node-token && echo ready || echo waiting")
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
		_, err = m.sudo(ctx, "k3s kubectl get nodes --request-timeout=5s 2>/dev/null")
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
	return m.sudo(ctx, "k3s kubectl get nodes -o wide -L pool -L kubernetes.io/arch")
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

	_, err = m.sudo(ctx, script)
	return err
}

// DrainNode drains a node before removal.
func (m *Manager) DrainNode(ctx context.Context, nodeName string) error {
	slog.Info("draining node", "host", m.runner.Host, "node", nodeName)
	fmt.Printf("  [%s] Draining node %s...\n", m.runner.Host, nodeName)
	_, err := m.sudo(ctx,
		fmt.Sprintf("k3s kubectl drain %s --ignore-daemonsets --delete-emptydir-data --force --timeout=120s", nodeName))
	return err
}

// DeleteNode removes a node from the cluster.
func (m *Manager) DeleteNode(ctx context.Context, nodeName string) error {
	slog.Info("deleting node from cluster", "host", m.runner.Host, "node", nodeName)
	fmt.Printf("  [%s] Deleting node %s from cluster...\n", m.runner.Host, nodeName)
	_, err := m.sudo(ctx,
		fmt.Sprintf("k3s kubectl delete node %s", nodeName))
	return err
}

// GetKubeconfig retrieves and patches the kubeconfig for external access.
func (m *Manager) GetKubeconfig(ctx context.Context, serverIP string) (string, error) {
	out, err := m.sudo(ctx, "cat /etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return "", err
	}
	// Replace localhost with the actual server IP.
	return strings.ReplaceAll(out, "127.0.0.1", serverIP), nil
}
