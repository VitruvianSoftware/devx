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
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/k3s"
	"github.com/VitruvianSoftware/devx/internal/multinode/lima"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// Apply iterates over all nodes, rolling VM config changes (CPUs, Memory, Disk,
// and host mounts) one node at a time for zero downtime. A node is only
// restarted when its live lima.yaml differs from the desired config, and the
// destructive cycle is gated behind a confirmation unless nonInteractive is set.
func Apply(ctx context.Context, cfg *config.Config, nonInteractive, dryRun bool) error {
	slog.Info("applying rolling updates to cluster", "name", cfg.Cluster.Name, "dry_run", dryRun)

	initNode := cfg.InitNode()
	initRunner := util.NewRunner(initNode)
	initK3s := k3s.NewManagerWithVM(initRunner, initNode.GetVMName())

	if err := initK3s.WaitForReady(ctx, 30*time.Second); err != nil {
		return fmt.Errorf("control plane not reachable on %s: %w", initNode.Host, err)
	}

	desiredMounts := lima.ResolveMounts(cfg.Cluster.Mounts)

	for _, node := range cfg.Nodes {
		runner := util.NewRunner(node)
		limaMgr := lima.NewManager(runner, node)
		vm := node.GetVMName()

		status, err := limaMgr.Status(ctx)
		if err != nil || status == lima.VMStatusNotCreated {
			slog.Info("skipping node (not provisioned)", "host", node.Host)
			continue
		}

		// Read the live lima.yaml (the file devx itself wrote) and diff.
		liveYAML, err := runner.RunShell(ctx, fmt.Sprintf("cat ~/.lima/%s/lima.yaml", vm))
		if err != nil {
			return fmt.Errorf("[%s] reading lima.yaml: %w", node.Host, err)
		}
		spec, err := lima.ParseSpec(liveYAML)
		if err != nil {
			return fmt.Errorf("[%s] parsing lima.yaml: %w", node.Host, err)
		}

		hwChanged := spec.CPUs != node.VM.CPUs || spec.Memory != node.VM.Memory || spec.Disk != node.VM.Disk
		mountsChanged := !lima.MountsEqual(spec.Mounts, desiredMounts)

		if !hwChanged && !mountsChanged {
			fmt.Printf("  [%s] up to date, skipping\n", node.Host)
			continue
		}

		fmt.Printf("\n🔄 Changes for %s:\n", node.Host)
		if hwChanged {
			fmt.Printf("    hardware: cpus=%d memory=%s disk=%s\n", node.VM.CPUs, node.VM.Memory, node.VM.Disk)
		}
		if mountsChanged {
			fmt.Printf("    mounts:   %d configured (was %d)\n", len(desiredMounts), len(spec.Mounts))
		}

		if dryRun {
			fmt.Printf("  [DRY RUN] Would drain %s, stop the VM, apply the above, and restart.\n", node.Host)
			continue
		}

		if !nonInteractive {
			fmt.Printf("  ⚠️  Applying restarts the VM and briefly disrupts Kubernetes on %s. Continue? [y/N] ", node.Host)
			var confirm string
			if _, err := fmt.Scanln(&confirm); err != nil || strings.ToLower(confirm) != "y" {
				fmt.Printf("  [%s] Skipped.\n", node.Host)
				continue
			}
		}

		// Step 1: Drain.
		fmt.Printf("  [%s] Draining Kubernetes node...\n", node.Host)
		if err := initK3s.DrainNode(ctx, node.Host); err != nil {
			slog.Warn("drain failed, but proceeding", "host", node.Host, "error", err)
		}

		// Step 2: Stop.
		fmt.Printf("  [%s] Stopping VM...\n", node.Host)
		if _, err = runner.RunShell(ctx, fmt.Sprintf("limactl stop %s", vm)); err != nil {
			return fmt.Errorf("[%s] stopping VM: %w", node.Host, err)
		}

		// Step 3a: Hardware limits (unchanged sed approach).
		if hwChanged {
			fmt.Printf("  [%s] Applying hardware limits (CPU=%d, Memory=%s, Disk=%s)...\n", node.Host, node.VM.CPUs, node.VM.Memory, node.VM.Disk)
			sedCmd := fmt.Sprintf("sed -i.bak -e 's/^cpus: .*/cpus: %d/' -e 's/^memory: .*/memory: \"%s\"/' -e 's/^disk: .*/disk: \"%s\"/' ~/.lima/%s/lima.yaml",
				node.VM.CPUs, node.VM.Memory, node.VM.Disk, vm)
			if _, err = runner.RunShell(ctx, sedCmd); err != nil {
				return fmt.Errorf("[%s] updating lima.yaml hardware: %w", node.Host, err)
			}
			fmt.Printf("  [%s] Resizing VM disk...\n", node.Host)
			if _, err = runner.RunShell(ctx, fmt.Sprintf("limactl disk resize %s --size %s", vm, node.VM.Disk)); err != nil {
				slog.Warn("disk resize failed", "host", node.Host, "error", err)
			}
		}

		// Step 3b: Mounts (base64-piped script to avoid quoting issues).
		if mountsChanged {
			fmt.Printf("  [%s] Applying host mounts...\n", node.Host)
			script := lima.MountsRewriteScript(vm, desiredMounts)
			cmd := fmt.Sprintf("echo %s | base64 -d | bash", util.Base64Encode(script))
			if _, err = runner.RunShell(ctx, cmd); err != nil {
				return fmt.Errorf("[%s] rewriting lima.yaml mounts: %w", node.Host, err)
			}
		}

		// Step 4: Restart.
		fmt.Printf("  [%s] Restarting VM...\n", node.Host)
		if _, err = runner.RunShell(ctx, fmt.Sprintf("limactl start %s", vm)); err != nil {
			return fmt.Errorf("[%s] starting VM: %w", node.Host, err)
		}

		// Step 5: Wait + uncordon.
		fmt.Printf("  [%s] Waiting for Kubernetes node to become ready...\n", node.Host)
		time.Sleep(10 * time.Second)
		fmt.Printf("  [%s] Uncordoning node...\n", node.Host)
		if err := initK3s.UncordonNode(ctx, node.Host); err != nil {
			return fmt.Errorf("[%s] uncordoning node: %w", node.Host, err)
		}

		fmt.Printf("  ✅ [%s] Update applied successfully.\n", node.Host)
	}

	fmt.Println("\n🎉 All nodes are up to date.")
	return nil
}
