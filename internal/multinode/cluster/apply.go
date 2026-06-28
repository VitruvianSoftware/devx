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

// nodeChanges reports whether a node's live lima.yaml spec differs from the
// desired hardware (cpus/memory/disk) and/or the desired mount set. A node is
// only restarted by Apply when at least one of these is true.
func nodeChanges(spec lima.Spec, vm config.VMConfig, desiredMounts []config.MountConfig) (hwChanged, mountsChanged bool) {
	hwChanged = spec.CPUs != vm.CPUs || spec.Memory != vm.Memory || spec.Disk != vm.Disk
	mountsChanged = !lima.MountsEqual(spec.Mounts, desiredMounts)
	return
}

// Apply iterates over all nodes, rolling VM config changes (CPUs, Memory, Disk,
// and host mounts) one node at a time for zero downtime. A node is only
// restarted when its live lima.yaml differs from the desired config, and the
// destructive cycle is gated behind a confirmation unless nonInteractive is set.
func Apply(ctx context.Context, cfg *config.Config, nonInteractive, dryRun bool) error {
	slog.Info("applying rolling updates to cluster", "name", cfg.Cluster.Name, "dry_run", dryRun)

	if !dryRun {
		lock, err := acquireClusterLock()
		if err != nil {
			return err
		}
		defer lock.release()
	}

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

		hwChanged, mountsChanged := nodeChanges(spec, node.VM, desiredMounts)

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

		// Drain → mutate VM → restart → wait Ready, with a GUARANTEED uncordon on
		// the way out (applyNodeCycle) so a mid-cycle failure can never strand the
		// node cordoned.
		if err := applyNodeCycle(ctx, initK3s, node.Host, func() error {
			// Stop the VM.
			fmt.Printf("  [%s] Stopping VM...\n", node.Host)
			if _, err := runner.RunShell(ctx, fmt.Sprintf("limactl stop %s", vm)); err != nil {
				return fmt.Errorf("stopping VM: %w", err)
			}

			// Hardware limits (sed) + disk resize.
			if hwChanged {
				fmt.Printf("  [%s] Applying hardware limits (CPU=%d, Memory=%s, Disk=%s)...\n", node.Host, node.VM.CPUs, node.VM.Memory, node.VM.Disk)
				sedCmd := fmt.Sprintf("sed -i.bak -e 's/^cpus: .*/cpus: %d/' -e 's/^memory: .*/memory: \"%s\"/' -e 's/^disk: .*/disk: \"%s\"/' ~/.lima/%s/lima.yaml",
					node.VM.CPUs, node.VM.Memory, node.VM.Disk, vm)
				if _, err := runner.RunShell(ctx, sedCmd); err != nil {
					return fmt.Errorf("updating lima.yaml hardware: %w", err)
				}
				fmt.Printf("  [%s] Resizing VM disk...\n", node.Host)
				if _, err := runner.RunShell(ctx, fmt.Sprintf("limactl disk resize %s --size %s", vm, node.VM.Disk)); err != nil {
					slog.Warn("disk resize failed", "host", node.Host, "error", err)
				}
			}

			// Mounts (base64-piped script to avoid quoting issues).
			if mountsChanged {
				fmt.Printf("  [%s] Applying host mounts...\n", node.Host)
				script := lima.MountsRewriteScript(vm, desiredMounts)
				cmd := fmt.Sprintf("echo %s | base64 -d | bash", util.Base64Encode(script))
				if _, err := runner.RunShell(ctx, cmd); err != nil {
					return fmt.Errorf("rewriting lima.yaml mounts: %w", err)
				}
			}

			// Restart.
			fmt.Printf("  [%s] Restarting VM...\n", node.Host)
			if _, err := runner.RunShell(ctx, fmt.Sprintf("limactl start %s", vm)); err != nil {
				return fmt.Errorf("starting VM: %w", err)
			}

			// Wait for the node to report Ready (replaces a fixed 10s sleep).
			fmt.Printf("  [%s] Waiting for Kubernetes node to become Ready...\n", node.Host)
			if err := waitNodeReady(ctx, func(c context.Context) (bool, error) {
				return initK3s.IsNodeReady(c, node.Host)
			}, 3*time.Minute, 5*time.Second); err != nil {
				return fmt.Errorf("node did not become Ready: %w", err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("[%s] applying update: %w", node.Host, err)
		}

		fmt.Printf("  ✅ [%s] Update applied successfully.\n", node.Host)
	}

	fmt.Println("\n🎉 All nodes are up to date.")
	return nil
}

// nodeCordoner is the subset of k3s operations applyNodeCycle needs; *k3s.Manager
// satisfies it. It is a seam so the uncordon-on-failure guarantee is unit
// testable without a live cluster.
type nodeCordoner interface {
	DrainNode(ctx context.Context, nodeName string) error
	UncordonNode(ctx context.Context, nodeName string) error
}

// applyNodeCycle drains the node, runs the VM-mutation body, and GUARANTEES an
// uncordon on the way out — so a failure mid stop/restart can never strand the
// node cordoned (the previous code returned early and left it cordoned). The
// uncordon is best-effort on a fresh bounded context so it runs even if ctx was
// cancelled.
func applyNodeCycle(ctx context.Context, ops nodeCordoner, host string, body func() error) error {
	fmt.Printf("  [%s] Draining Kubernetes node...\n", host)
	if err := ops.DrainNode(ctx, host); err != nil {
		slog.Warn("drain failed, but proceeding", "host", host, "error", err)
	}
	defer func() {
		uctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fmt.Printf("  [%s] Uncordoning node...\n", host)
		if err := ops.UncordonNode(uctx, host); err != nil {
			slog.Warn("uncordon failed", "host", host, "error", err)
		}
	}()
	return body()
}

// waitNodeReady polls ready() until it reports true or timeout elapses, checking
// every interval. It replaces a fixed sleep before uncordon so Apply waits for
// the node to actually rejoin rather than guessing.
func waitNodeReady(ctx context.Context, ready func(context.Context) (bool, error), timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if ok, err := ready(ctx); err == nil && ok {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timed out after %s waiting for node to become Ready", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
