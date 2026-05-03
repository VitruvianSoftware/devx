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

// Package cluster orchestrates the full lifecycle of a multi-node K8s cluster.
// It coordinates Lima VM provisioning, K3s installation, and cluster operations
// across multiple remote macOS hosts.
package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/k3s"
	"github.com/VitruvianSoftware/devx/internal/multinode/lima"
	"github.com/VitruvianSoftware/devx/internal/multinode/prereqs"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// InitOptions configures the Init operation.
type InitOptions struct {
	DryRun      bool
	AutoInstall bool
}

// Init bootstraps a new cluster from scratch. It is idempotent.
func Init(ctx context.Context, cfg *config.Config, opts InitOptions) error {
	slog.Info("initializing multi-node cluster", "name", cfg.Cluster.Name, "nodes", len(cfg.Nodes), "dry_run", opts.DryRun)

	if opts.DryRun {
		printDryRunPlan(cfg)
		return nil
	}

	// Phase 0: Ensure prerequisites on all hosts (parallel).
	fmt.Println("🔍 Phase 0: Checking prerequisites...")
	g, gctx := errgroup.WithContext(ctx)
	for _, node := range cfg.Nodes {
		node := node
		g.Go(func() error {
			return prereqs.EnsureAll(gctx, node, opts.AutoInstall)
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("prerequisite check failed: %w", err)
	}

	// Phase 1: Provision all Lima VMs (parallel).
	fmt.Println("\n📦 Phase 1: Provisioning VMs (parallel)...")
	var mu sync.Mutex
	ipMap := make(map[string]string)

	g, gctx = errgroup.WithContext(ctx)
	for i, node := range cfg.Nodes {
		node := node
		idx := i + 1
		total := len(cfg.Nodes)
		g.Go(func() error {
			runner := util.NewRunner(node)
			mgr := lima.NewManager(runner, node)

			if err := mgr.Provision(gctx); err != nil {
				return fmt.Errorf("[%s] provisioning VM: %w", node.Host, err)
			}

			ip, err := mgr.GetBridgedIP(gctx)
			if err != nil {
				return fmt.Errorf("[%s] getting bridged IP: %w", node.Host, err)
			}

			mu.Lock()
			ipMap[node.Host] = ip
			mu.Unlock()

			fmt.Printf("  [%d/%d] %s: VM ready (IP: %s)\n", idx, total, node.Host, ip)
			slog.Info("VM provisioned", "host", node.Host, "ip", ip)
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return fmt.Errorf("VM provisioning failed: %w\n\nRecovery: re-run 'cluster init' to retry", err)
	}

	// Phase 2: Validate cross-VM network connectivity.
	fmt.Println("\n🌐 Phase 2: Validating network connectivity...")
	if err := validateNetwork(ctx, cfg, ipMap); err != nil {
		return err
	}

	// Collect all server IPs for TLS SANs.
	var serverIPs []string
	for _, node := range cfg.ServerNodes() {
		serverIPs = append(serverIPs, ipMap[node.Host])
	}

	// Phase 3: Initialize the first control plane node.
	fmt.Println("\n🔧 Phase 3: Bootstrapping control plane (CP-1)...")
	initNode := cfg.InitNode()
	initRunner := util.NewRunner(initNode)
	initK3s := k3s.NewManagerWithVM(initRunner, initNode.GetVMName())

	if err := initK3s.InitCluster(ctx, ipMap[initNode.Host], initNode.Pool, cfg.Cluster.K3sVersion, serverIPs); err != nil {
		return fmt.Errorf("[%s] initializing K3s: %w\n\nRecovery: re-run 'cluster init' to retry", initNode.Host, err)
	}

	if err := initK3s.WaitForReady(ctx, 5*time.Minute); err != nil {
		return err
	}

	token, err := initK3s.GetToken(ctx)
	if err != nil {
		return fmt.Errorf("[%s] getting token: %w", initNode.Host, err)
	}
	slog.Info("cluster initialized, token acquired", "host", initNode.Host)

	serverURL := fmt.Sprintf("https://%s:6443", ipMap[initNode.Host])

	// Phase 4: Join remaining server nodes.
	servers := cfg.ServerNodes()
	if len(servers) > 1 {
		fmt.Println("\n🔗 Phase 4: Joining control plane nodes...")
		for _, node := range servers[1:] {
			runner := util.NewRunner(node)
			k3sMgr := k3s.NewManagerWithVM(runner, node.GetVMName())

			if err := k3sMgr.JoinServer(ctx, ipMap[node.Host], serverURL, token, node.Pool, cfg.Cluster.K3sVersion, []string{ipMap[node.Host]}); err != nil {
				return fmt.Errorf("[%s] joining as server: %w\n\nRecovery: re-run 'cluster init' to retry", node.Host, err)
			}

			if err := k3sMgr.WaitForReady(ctx, 5*time.Minute); err != nil {
				return err
			}
			slog.Info("node joined as server", "host", node.Host)
		}
	}

	// Phase 5: Join agent nodes.
	agents := cfg.AgentNodes()
	if len(agents) > 0 {
		fmt.Println("\n🔗 Phase 5: Joining worker nodes...")
		for _, node := range agents {
			runner := util.NewRunner(node)
			k3sMgr := k3s.NewManagerWithVM(runner, node.GetVMName())

			if err := k3sMgr.JoinAgent(ctx, ipMap[node.Host], serverURL, token, node.Pool, cfg.Cluster.K3sVersion); err != nil {
				return fmt.Errorf("[%s] joining as agent: %w\n\nRecovery: re-run 'cluster init' to retry", node.Host, err)
			}
			slog.Info("node joined as agent", "host", node.Host)
		}
	}

	// Phase 6: Export kubeconfig.
	fmt.Println("\n📋 Phase 6: Exporting kubeconfig...")
	if err := exportKubeconfig(ctx, initK3s, ipMap[initNode.Host], cfg.Cluster.Kubeconfig); err != nil {
		return err
	}

	// Phase 7: Final validation.
	fmt.Println("\n✅ Phase 7: Validating cluster...")
	out, err := initK3s.GetNodeStatus(ctx)
	if err != nil {
		return fmt.Errorf("[%s] getting node status: %w", initNode.Host, err)
	}
	fmt.Println(out)
	fmt.Printf("\n🎉 Cluster %q is ready!\n", cfg.Cluster.Name)

	return nil
}

// Join adds nodes that are in the config but not yet in the cluster.
func Join(ctx context.Context, cfg *config.Config, dryRun bool) error {
	slog.Info("joining new nodes to cluster", "name", cfg.Cluster.Name, "dry_run", dryRun)

	var (
		initIP string
		token  string
		found  bool
	)

	// Find the first healthy server node to query current cluster state and retrieve the token.
	for _, n := range cfg.ServerNodes() {
		runner := util.NewRunner(n)
		k3sMgr := k3s.NewManagerWithVM(runner, n.GetVMName())

		installed, _ := k3sMgr.IsInstalled(ctx)
		if !installed {
			continue
		}

		limaMgr := lima.NewManager(runner, n)
		ip, err := limaMgr.GetBridgedIP(ctx)
		if err != nil {
			slog.Debug("failed to get IP for healthy server", "host", n.Host, "error", err)
			continue
		}

		t, err := k3sMgr.GetToken(ctx)
		if err != nil {
			slog.Debug("failed to get token from healthy server", "host", n.Host, "error", err)
			continue
		}

		initIP = ip
		token = t
		found = true
		break
	}

	if !found {
		return fmt.Errorf("no existing healthy server node found to join against")
	}

	serverURL := fmt.Sprintf("https://%s:6443", initIP)

	for _, node := range cfg.Nodes {
		runner := util.NewRunner(node)
		k3sMgr := k3s.NewManagerWithVM(runner, node.GetVMName())
		limaMgr := lima.NewManager(runner, node)

		installed, _ := k3sMgr.IsInstalled(ctx)
		if installed {
			slog.Debug("node already part of cluster, skipping", "host", node.Host)
			continue
		}

		if dryRun {
			fmt.Printf("  [%s] Would join as %s (pool=%s)\n", node.Host, node.Role, node.Pool)
			continue
		}

		// Ensure VM is provisioned.
		if err := limaMgr.Provision(ctx); err != nil {
			return fmt.Errorf("[%s] provisioning: %w", node.Host, err)
		}

		nodeIP, err := limaMgr.GetBridgedIP(ctx)
		if err != nil {
			return fmt.Errorf("[%s] getting IP: %w", node.Host, err)
		}

		switch node.Role {
		case "server":
			if err := k3sMgr.JoinServer(ctx, nodeIP, serverURL, token, node.Pool, cfg.Cluster.K3sVersion, []string{nodeIP}); err != nil {
				return fmt.Errorf("[%s] joining as server: %w", node.Host, err)
			}
		case "agent":
			if err := k3sMgr.JoinAgent(ctx, nodeIP, serverURL, token, node.Pool, cfg.Cluster.K3sVersion); err != nil {
				return fmt.Errorf("[%s] joining as agent: %w", node.Host, err)
			}
		}

		slog.Info("node joined", "host", node.Host, "role", node.Role)
	}

	return nil
}

// Remove drains and removes a specific node from the cluster.
func Remove(ctx context.Context, cfg *config.Config, hostName string, dryRun bool) error {
	slog.Info("removing node", "host", hostName, "dry_run", dryRun)

	// Find the node in the config.
	var target *config.NodeConfig
	for _, n := range cfg.Nodes {
		n := n
		if n.Host == hostName {
			target = &n
			break
		}
	}
	if target == nil {
		return fmt.Errorf("node %q not found in config", hostName)
	}

	if dryRun {
		fmt.Printf("  Would drain and remove %s (%s) from cluster\n", hostName, target.Role)
		return nil
	}

	// Use the init node to drain and delete from the cluster.
	initNode := cfg.InitNode()
	initRunner := util.NewRunner(initNode)
	initK3s := k3s.NewManagerWithVM(initRunner, initNode.GetVMName())

	// Drain the node.
	if err := initK3s.DrainNode(ctx, hostName); err != nil {
		slog.Warn("drain failed (may already be removed)", "host", hostName, "error", err)
	}

	// Delete the node from K8s.
	if err := initK3s.DeleteNode(ctx, hostName); err != nil {
		slog.Warn("delete failed (may already be removed)", "host", hostName, "error", err)
	}

	// Uninstall K3s on the target.
	targetRunner := util.NewRunner(*target)
	targetK3s := k3s.NewManagerWithVM(targetRunner, target.GetVMName())
	if err := targetK3s.Uninstall(ctx, target.Role); err != nil {
		return fmt.Errorf("[%s] uninstalling K3s: %w", target.Host, err)
	}

	slog.Info("node removed successfully", "host", hostName)
	return nil
}

// Destroy tears down the entire cluster.
func Destroy(ctx context.Context, cfg *config.Config, force, dryRun bool) error {
	slog.Info("destroying cluster", "name", cfg.Cluster.Name, "force", force, "dry_run", dryRun)

	if dryRun {
		fmt.Printf("Would destroy cluster %q:\n", cfg.Cluster.Name)
		for _, node := range cfg.Nodes {
			fmt.Printf("  [%s] Would uninstall K3s (%s) and destroy VM\n", node.Host, node.Role)
		}
		return nil
	}

	if !force {
		fmt.Printf("⚠️  This will destroy the entire cluster %q. Are you sure? [y/N] ", cfg.Cluster.Name)
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil || strings.ToLower(confirm) != "y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	fmt.Println("💥 Destroying cluster:", cfg.Cluster.Name)

	// Uninstall K3s and destroy VMs on all nodes (agents first, then servers in reverse).
	allNodes := append(cfg.AgentNodes(), reverseNodes(cfg.ServerNodes())...)

	for _, node := range allNodes {
		runner := util.NewRunner(node)
		k3sMgr := k3s.NewManagerWithVM(runner, node.GetVMName())
		limaMgr := lima.NewManager(runner, node)

		// Uninstall K3s.
		if err := k3sMgr.Uninstall(ctx, node.Role); err != nil {
			slog.Warn("K3s uninstall failed", "host", node.Host, "error", err)
		}

		// Destroy the Lima VM.
		if err := limaMgr.Destroy(ctx); err != nil {
			slog.Warn("VM destroy failed", "host", node.Host, "error", err)
		}
	}

	// Remove kubeconfig.
	kubeconfigPath := expandPath(cfg.Cluster.Kubeconfig)
	if kubeconfigPath != "" {
		if err := os.Remove(kubeconfigPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to remove kubeconfig", "path", kubeconfigPath, "error", err)
		} else if err == nil {
			slog.Info("removed kubeconfig", "path", kubeconfigPath)
		}
	}

	fmt.Printf("\n🗑️  Cluster %q destroyed.\n", cfg.Cluster.Name)
	return nil
}

// Status displays the current state of all configured nodes.
func Status(ctx context.Context, cfg *config.Config) error {
	slog.Info("checking cluster status", "name", cfg.Cluster.Name)

	fmt.Println("📊 Cluster status:", cfg.Cluster.Name)
	fmt.Println()

	for _, node := range cfg.Nodes {
		runner := util.NewRunner(node)
		limaMgr := lima.NewManager(runner, node)
		k3sMgr := k3s.NewManagerWithVM(runner, node.GetVMName())

		vmStatus, _ := limaMgr.Status(ctx)
		k3sInstalled, _ := k3sMgr.IsInstalled(ctx)

		k3sStatus := "not installed"
		if k3sInstalled {
			k3sStatus = "installed"
		}

		fmt.Printf("  %-25s role=%-6s pool=%-15s vm=%-12s k3s=%s\n",
			node.Host, node.Role, node.Pool, string(vmStatus), k3sStatus)
	}

	// If any server node is reachable, show kubectl output.
	fmt.Println()
	initNode := cfg.InitNode()
	initRunner := util.NewRunner(initNode)
	initK3s := k3s.NewManagerWithVM(initRunner, initNode.GetVMName())
	out, err := initK3s.GetNodeStatus(ctx)
	if err == nil {
		fmt.Println(out)
	} else {
		fmt.Println("  (Cluster not reachable for node status)")
	}

	return nil
}

// --- helpers ---

// validateNetwork performs a cross-VM ping matrix to ensure all nodes can communicate.
func validateNetwork(ctx context.Context, cfg *config.Config, ipMap map[string]string) error {
	var failures []string

	for _, from := range cfg.Nodes {
		runner := util.NewRunner(from)
		for _, to := range cfg.Nodes {
			if from.Host == to.Host {
				continue
			}
			toIP := ipMap[to.Host]
			_, err := runner.LimaShell(ctx, from.GetVMName(), fmt.Sprintf("ping -c 1 -W 3 %s", toIP))
			if err != nil {
				failures = append(failures, fmt.Sprintf("  %s → %s (%s): FAILED", from.Host, to.Host, toIP))
				slog.Warn("network check failed", "from", from.Host, "to", to.Host, "to_ip", toIP)
			} else {
				slog.Debug("network check passed", "from", from.Host, "to", to.Host, "to_ip", toIP)
			}
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("network validation failed — the following paths are broken:\n%s\n\nEnsure all VMs are bridged to the same LAN via socket_vmnet", strings.Join(failures, "\n"))
	}

	fmt.Println("  ✅ All VMs can reach each other")
	return nil
}

func exportKubeconfig(ctx context.Context, k3sMgr *k3s.Manager, serverIP, kubeconfigPath string) error {
	kubeconfig, err := k3sMgr.GetKubeconfig(ctx, serverIP)
	if err != nil {
		return fmt.Errorf("getting kubeconfig: %w", err)
	}

	outPath := expandPath(kubeconfigPath)
	if outPath == "" {
		outPath = expandPath("~/.kube/config")
	}

	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating kubeconfig directory: %w", err)
	}

	if err := os.WriteFile(outPath, []byte(kubeconfig), 0o600); err != nil {
		return fmt.Errorf("writing kubeconfig: %w", err)
	}

	slog.Info("kubeconfig written", "path", outPath)
	return nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func reverseNodes(nodes []config.NodeConfig) []config.NodeConfig {
	reversed := make([]config.NodeConfig, len(nodes))
	for i, n := range nodes {
		reversed[len(nodes)-1-i] = n
	}
	return reversed
}

func printDryRunPlan(cfg *config.Config) {
	fmt.Println("\n📋 Dry-run plan:")
	fmt.Println()
	for _, node := range cfg.ServerNodes() {
		role := "server"
		if node.Host == cfg.InitNode().Host {
			role = "server (init)"
		}
		fmt.Printf("  [%s] Provision VM → Install K3s %s → Label pool=%s\n", node.Host, role, node.Pool)
	}
	for _, node := range cfg.AgentNodes() {
		fmt.Printf("  [%s] Provision VM → Install K3s agent → Label pool=%s\n", node.Host, node.Pool)
	}
	fmt.Println("\n  No changes were made (dry-run mode).")
}
