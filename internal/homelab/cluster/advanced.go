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

package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/k3s"
	"github.com/VitruvianSoftware/devx/internal/homelab/lima"
	"github.com/VitruvianSoftware/devx/internal/homelab/util"
)

// Upgrade performs a rolling upgrade of K3s across all nodes.
// Agents are upgraded first, then servers one at a time to maintain quorum.
func Upgrade(ctx context.Context, cfg *config.Config, dryRun bool) error {
	targetVersion := cfg.Cluster.K3sVersion
	if targetVersion == "" {
		return fmt.Errorf("cluster.k3sVersion is not set in config")
	}

	slog.Info("starting rolling upgrade", "target_version", targetVersion, "dry_run", dryRun)
	fmt.Printf("🔄 Rolling upgrade to K3s %s\n", targetVersion)

	initNode := cfg.InitNode()
	initRunner := util.NewRunner(initNode)
	initK3s := k3s.NewManagerWithVM(initRunner, initNode.GetVMName())

	// Phase 1: Upgrade agent nodes first.
	agents := cfg.AgentNodes()
	if len(agents) > 0 {
		fmt.Println("\n📦 Phase 1: Upgrading agent nodes...")
		for _, node := range agents {
			if err := upgradeNode(ctx, cfg, initK3s, node, targetVersion, "agent", dryRun); err != nil {
				return err
			}
		}
	}

	// Phase 2: Upgrade server nodes one at a time (skip init node, do it last).
	servers := cfg.ServerNodes()
	if len(servers) > 1 {
		fmt.Println("\n📦 Phase 2: Upgrading non-init server nodes...")
		for _, node := range servers[1:] {
			if err := upgradeNode(ctx, cfg, initK3s, node, targetVersion, "server", dryRun); err != nil {
				return err
			}
		}
	}

	// Phase 3: Upgrade the init node last.
	fmt.Println("\n📦 Phase 3: Upgrading init server node...")
	if err := upgradeNode(ctx, cfg, initK3s, initNode, targetVersion, "server", dryRun); err != nil {
		return err
	}

	// Phase 4: Validate.
	if !dryRun {
		fmt.Println("\n✅ Phase 4: Validating cluster...")
		out, err := initK3s.GetNodeStatus(ctx)
		if err != nil {
			return fmt.Errorf("[%s] getting node status: %w", initNode.Host, err)
		}
		fmt.Println(out)
	}

	fmt.Printf("\n🎉 Upgrade to K3s %s complete!\n", targetVersion)
	return nil
}

func upgradeNode(ctx context.Context, cfg *config.Config, initK3s *k3s.Manager, node config.NodeConfig, version, role string, dryRun bool) error {
	runner := util.NewRunner(node)
	k3sMgr := k3s.NewManagerWithVM(runner, node.GetVMName())

	// Check current version.
	currentVersion, err := k3sMgr.GetVersion(ctx)
	if err != nil {
		slog.Warn("could not determine current K3s version", "host", node.Host, "error", err)
	}

	if currentVersion == version {
		fmt.Printf("  [%s] Already at %s, skipping\n", node.Host, version)
		return nil
	}

	if dryRun {
		fmt.Printf("  [%s] Would upgrade %s → %s\n", node.Host, currentVersion, version)
		return nil
	}

	// Drain the node.
	slog.Info("draining node for upgrade", "host", node.Host)
	fmt.Printf("  [%s] Draining...\n", node.Host)
	if err := initK3s.DrainNode(ctx, node.Host); err != nil {
		slog.Warn("drain failed, continuing", "host", node.Host, "error", err)
	}

	// Reinstall K3s with the new version.
	slog.Info("upgrading K3s", "host", node.Host, "from", currentVersion, "to", version)
	fmt.Printf("  [%s] Upgrading K3s %s → %s...\n", node.Host, currentVersion, version)

	limaMgr := lima.NewManager(runner, node)
	nodeIP, err := limaMgr.GetBridgedIP(ctx)
	if err != nil {
		return fmt.Errorf("[%s] getting IP: %w", node.Host, err)
	}

	var installErr error
	switch role {
	case "server":
		if node.Host == cfg.InitNode().Host {
			var serverIPs []string
			for _, s := range cfg.ServerNodes() {
				sRunner := util.NewRunner(s)
				sLima := lima.NewManager(sRunner, s)
				ip, _ := sLima.GetBridgedIP(ctx)
				if ip != "" {
					serverIPs = append(serverIPs, ip)
				}
			}
			installErr = k3sMgr.ReinstallServer(ctx, nodeIP, version, serverIPs, node.Pool)
		} else {
			serverURL := fmt.Sprintf("https://%s:6443", getInitIP(ctx, cfg))
			token, _ := initK3s.GetToken(ctx)
			installErr = k3sMgr.ReinstallJoinServer(ctx, nodeIP, serverURL, token, version, node.Pool)
		}
	case "agent":
		serverURL := fmt.Sprintf("https://%s:6443", getInitIP(ctx, cfg))
		token, _ := initK3s.GetToken(ctx)
		installErr = k3sMgr.ReinstallAgent(ctx, nodeIP, serverURL, token, version, node.Pool)
	}

	if installErr != nil {
		return fmt.Errorf("[%s] upgrade failed: %w", node.Host, installErr)
	}

	// Wait for the node to become ready.
	if role == "server" {
		if err := k3sMgr.WaitForReady(ctx, 5*time.Minute); err != nil {
			return err
		}
	}

	// Uncordon the node.
	slog.Info("uncordoning node", "host", node.Host)
	fmt.Printf("  [%s] Uncordoning...\n", node.Host)
	if err := initK3s.UncordonNode(ctx, node.Host); err != nil {
		slog.Warn("uncordon failed", "host", node.Host, "error", err)
	}

	// Health check.
	time.Sleep(5 * time.Second) // brief settle time
	out, err := initK3s.GetNodeStatus(ctx)
	if err != nil {
		slog.Warn("post-upgrade health check failed", "host", node.Host, "error", err)
	} else if strings.Contains(out, "NotReady") {
		slog.Warn("some nodes NotReady after upgrade", "host", node.Host)
	}

	fmt.Printf("  [%s] ✅ Upgraded successfully\n", node.Host)
	return nil
}

// Backup creates an etcd snapshot on the init node and downloads it locally.
func Backup(ctx context.Context, cfg *config.Config, outputDir string) error {
	initNode := cfg.InitNode()
	initRunner := util.NewRunner(initNode)
	initK3s := k3s.NewManagerWithVM(initRunner, initNode.GetVMName())

	slog.Info("creating etcd snapshot", "host", initNode.Host)
	fmt.Printf("📸 Creating etcd snapshot on %s...\n", initNode.Host)

	snapshotName, err := initK3s.CreateSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("[%s] creating snapshot: %w", initNode.Host, err)
	}

	fmt.Printf("  Snapshot created: %s\n", snapshotName)

	// Download the snapshot.
	localPath := filepath.Join(outputDir, filepath.Base(snapshotName))
	if err := initK3s.DownloadSnapshot(ctx, snapshotName, localPath); err != nil {
		return fmt.Errorf("[%s] downloading snapshot: %w", initNode.Host, err)
	}

	fmt.Printf("  📁 Saved to: %s\n", localPath)
	return nil
}

// Restore restores etcd from a snapshot file.
func Restore(ctx context.Context, cfg *config.Config, snapshotPath string) error {
	initNode := cfg.InitNode()
	initRunner := util.NewRunner(initNode)
	initK3s := k3s.NewManagerWithVM(initRunner, initNode.GetVMName())

	slog.Info("restoring from snapshot", "host", initNode.Host, "snapshot", snapshotPath)
	fmt.Printf("🔄 Restoring etcd from %s on %s...\n", snapshotPath, initNode.Host)

	// Upload the snapshot.
	remotePath := "/tmp/homelab-restore-snapshot"
	if err := initK3s.UploadSnapshot(ctx, snapshotPath, remotePath); err != nil {
		return fmt.Errorf("[%s] uploading snapshot: %w", initNode.Host, err)
	}

	// Stop K3s.
	fmt.Println("  Stopping K3s...")
	if err := initK3s.StopService(ctx); err != nil {
		return fmt.Errorf("[%s] stopping K3s: %w", initNode.Host, err)
	}

	// Restore.
	if err := initK3s.RestoreSnapshot(ctx, remotePath); err != nil {
		return fmt.Errorf("[%s] restoring snapshot: %w", initNode.Host, err)
	}

	// Restart K3s.
	fmt.Println("  Restarting K3s...")
	if err := initK3s.StartService(ctx); err != nil {
		return fmt.Errorf("[%s] starting K3s: %w", initNode.Host, err)
	}

	// Wait for ready.
	if err := initK3s.WaitForReady(ctx, 5*time.Minute); err != nil {
		return err
	}

	fmt.Println("  ✅ Restore complete!")
	return nil
}

func getInitIP(ctx context.Context, cfg *config.Config) string {
	initNode := cfg.InitNode()
	runner := util.NewRunner(initNode)
	mgr := lima.NewManager(runner, initNode)
	ip, _ := mgr.GetBridgedIP(ctx)
	return ip
}
