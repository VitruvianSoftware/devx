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

// Package doctor provides pre-flight and health checks for the multi-node cluster.
package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/k3s"
	"github.com/VitruvianSoftware/devx/internal/multinode/lima"
	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// CheckResult represents the outcome of a single diagnostic check.
type CheckResult struct {
	Name    string
	Host    string
	Passed  bool
	Message string
}

// Run executes all diagnostic checks and reports the results.
func Run(ctx context.Context, cfg *config.Config, fix bool) error {
	fmt.Println("🩺 Running diagnostics for cluster:", cfg.Cluster.Name)
	if fix {
		fmt.Println("   (Auto-fix enabled: attempting to resolve detected issues)")
	}
	fmt.Println()

	var results []CheckResult
	var failed int

	for _, node := range cfg.Nodes {
		runner := util.NewRunner(node)

		// Check 1: SSH connectivity.
		results = append(results, checkSSH(ctx, runner, node.Host))

		// Check 2: Homebrew.
		results = append(results, checkBrew(ctx, runner, node.Host))

		// Check 3: Lima.
		results = append(results, checkLima(ctx, runner, node.Host))

		// Check 4: socket_vmnet.
		results = append(results, checkSocketVmnet(ctx, runner, node.Host, fix))

		// Check 5: VM status.
		limaMgr := lima.NewManager(runner, node)
		results = append(results, checkVM(ctx, limaMgr, runner, node.GetVMName(), node.Host, fix))

		// Check 6: Bridged IP.
		results = append(results, checkBridgedIP(ctx, limaMgr, runner, node.GetVMName(), node.Host, fix))

		k3sMgr := k3s.NewManagerWithVM(runner, node.GetVMName())
		results = append(results, checkK3s(ctx, k3sMgr, runner, node.GetVMName(), node.Host, node.Role, fix))
	}

	// Check 8: Cross-VM connectivity (if multiple VMs are running).
	ips := collectBridgedIPs(ctx, cfg)
	if len(ips) > 1 {
		for _, node := range cfg.Nodes {
			host := node.Host
			_, ok := ips[host]
			if !ok {
				continue
			}
			for _, otherNode := range cfg.Nodes {
				otherHost := otherNode.Host
				otherIP, otherOk := ips[otherHost]
				if host == otherHost || !otherOk {
					continue
				}
				runner := util.NewRunner(node)
				results = append(results, checkPing(ctx, runner, node.GetVMName(), host, otherHost, otherIP))
			}
		}
	}

	// Check 9: Cluster health (from init node).
	initNode := cfg.InitNode()
	initRunner := util.NewRunner(initNode)
	initK3s := k3s.NewManagerWithVM(initRunner, initNode.GetVMName())
	results = append(results, checkClusterHealth(ctx, initK3s, initRunner, initNode.GetVMName(), initNode.Host, cfg, fix))

	// Print results.
	fmt.Println()
	for _, r := range results {
		icon := "✅"
		if !r.Passed {
			icon = "❌"
			failed++
		}
		fmt.Printf("  %s [%-25s] %s: %s\n", icon, r.Host, r.Name, r.Message)
	}

	fmt.Println()
	total := len(results)
	passed := total - failed
	fmt.Printf("  Results: %d/%d checks passed\n", passed, total)

	if failed > 0 {
		return fmt.Errorf("%d check(s) failed", failed)
	}

	fmt.Println("\n  🎉 All checks passed!")
	return nil
}

func checkSSH(ctx context.Context, runner *remote.Runner, host string) CheckResult {
	_, err := runner.Run(ctx, "echo ok")
	if err != nil {
		return CheckResult{Name: "SSH", Host: host, Passed: false, Message: fmt.Sprintf("connection failed: %v", err)}
	}
	return CheckResult{Name: "SSH", Host: host, Passed: true, Message: "connected"}
}

func checkBrew(ctx context.Context, runner *remote.Runner, host string) CheckResult {
	_, err := runner.Run(ctx, "which brew || ls /opt/homebrew/bin/brew")
	if err != nil {
		return CheckResult{Name: "Homebrew", Host: host, Passed: false, Message: "not found"}
	}
	return CheckResult{Name: "Homebrew", Host: host, Passed: true, Message: "installed"}
}

func checkLima(ctx context.Context, runner *remote.Runner, host string) CheckResult {
	out, err := runner.Run(ctx, "limactl --version 2>/dev/null || echo not-found")
	if err != nil || strings.Contains(out, "not-found") {
		return CheckResult{Name: "Lima", Host: host, Passed: false, Message: "not installed"}
	}
	return CheckResult{Name: "Lima", Host: host, Passed: true, Message: strings.TrimSpace(out)}
}

func checkSocketVmnet(ctx context.Context, runner *remote.Runner, host string, fix bool) CheckResult {
	cmd := `if sudo test -S /opt/homebrew/var/run/socket_vmnet; then echo /opt/homebrew/var/run/socket_vmnet; elif sudo test -S /usr/local/var/run/socket_vmnet; then echo /usr/local/var/run/socket_vmnet; elif sudo test -S /var/run/socket_vmnet; then echo /var/run/socket_vmnet; else exit 1; fi`
	out, err := runner.RunShell(ctx, cmd)
	if err != nil {
		if fix {
			fmt.Printf("  [%s] 🔧 Auto-fixing socket_vmnet (restarting daemon)...\n", host)
			// Try to restart via homebrew services on macOS
			_, _ = runner.RunShell(ctx, "sudo launchctl stop homebrew.mxcl.socket_vmnet 2>/dev/null || true")
			_, _ = runner.RunShell(ctx, "sudo launchctl start homebrew.mxcl.socket_vmnet")

			out, err = runner.RunShell(ctx, cmd)
			if err == nil {
				return CheckResult{Name: "socket_vmnet", Host: host, Passed: true, Message: fmt.Sprintf("running at %s (fixed)", strings.TrimSpace(out))}
			}
		}
		return CheckResult{Name: "socket_vmnet", Host: host, Passed: false, Message: "socket not found"}
	}
	return CheckResult{Name: "socket_vmnet", Host: host, Passed: true, Message: fmt.Sprintf("running at %s", strings.TrimSpace(out))}
}

func checkVM(ctx context.Context, mgr *lima.Manager, runner *remote.Runner, vmName, host string, fix bool) CheckResult {
	status, err := mgr.Status(ctx)
	if err != nil {
		return CheckResult{Name: "Lima VM", Host: host, Passed: false, Message: fmt.Sprintf("error: %v", err)}
	}
	passed := status == lima.VMStatusRunning
	if !passed && fix {
		fmt.Printf("  [%s] 🔧 Auto-fixing VM (starting VM)...\n", host)
		_, _ = runner.RunShell(ctx, fmt.Sprintf("PATH=$PATH:/usr/local/bin:/opt/homebrew/bin LIMA_HOME=~/.lima limactl start %s", vmName))
		status, _ = mgr.Status(ctx)
		passed = status == lima.VMStatusRunning
		if passed {
			return CheckResult{Name: "Lima VM", Host: host, Passed: true, Message: "Running (fixed)"}
		}
	}
	return CheckResult{Name: "Lima VM", Host: host, Passed: passed, Message: string(status)}
}

func checkBridgedIP(ctx context.Context, mgr *lima.Manager, runner *remote.Runner, vmName, host string, fix bool) CheckResult {
	ip, err := mgr.GetBridgedIP(ctx)
	if err != nil {
		if fix {
			fmt.Printf("  [%s] 🔧 Auto-fixing Bridged IP (restarting VM to fetch DHCP lease)...\n", host)
			_, _ = runner.RunShell(ctx, fmt.Sprintf("PATH=$PATH:/usr/local/bin:/opt/homebrew/bin LIMA_HOME=~/.lima limactl stop %s", vmName))
			_, _ = runner.RunShell(ctx, fmt.Sprintf("PATH=$PATH:/usr/local/bin:/opt/homebrew/bin LIMA_HOME=~/.lima limactl start %s", vmName))
			ip, err = mgr.GetBridgedIP(ctx)
			if err == nil {
				return CheckResult{Name: "Bridged IP", Host: host, Passed: true, Message: fmt.Sprintf("%s (fixed)", ip)}
			}
		}
		return CheckResult{Name: "Bridged IP", Host: host, Passed: false, Message: fmt.Sprintf("not available: %v", err)}
	}
	return CheckResult{Name: "Bridged IP", Host: host, Passed: true, Message: ip}
}

func checkK3s(ctx context.Context, mgr *k3s.Manager, runner *remote.Runner, vmName, host string, role string, fix bool) CheckResult {
	installed, err := mgr.IsInstalled(ctx)
	if err != nil {
		return CheckResult{Name: "K3s", Host: host, Passed: false, Message: fmt.Sprintf("error: %v", err)}
	}
	if !installed {
		return CheckResult{Name: "K3s", Host: host, Passed: false, Message: "not installed"}
	}

	// Check if active
	serviceName := "k3s"
	if role == "agent" {
		serviceName = "k3s-agent"
	}

	out, err := runner.LimaShellSudo(ctx, vmName, fmt.Sprintf("systemctl is-active %s 2>/dev/null", serviceName))
	active := err == nil && strings.TrimSpace(out) == "active"

	if !active && fix {
		fmt.Printf("  [%s] 🔧 Auto-fixing K3s (restarting %s)...\n", host, serviceName)
		_, _ = runner.LimaShellSudo(ctx, vmName, fmt.Sprintf("systemctl restart %s", serviceName))
		out, err = runner.LimaShellSudo(ctx, vmName, fmt.Sprintf("systemctl is-active %s 2>/dev/null", serviceName))
		if err == nil && strings.TrimSpace(out) == "active" {
			return CheckResult{Name: "K3s", Host: host, Passed: true, Message: "installed & active (fixed)"}
		}
		return CheckResult{Name: "K3s", Host: host, Passed: false, Message: "installed but inactive (fix failed)"}
	}

	if !active {
		return CheckResult{Name: "K3s", Host: host, Passed: false, Message: "installed but inactive"}
	}

	return CheckResult{Name: "K3s", Host: host, Passed: true, Message: "installed & active"}
}

func checkPing(ctx context.Context, runner *remote.Runner, vmName, fromHost, toHost, toIP string) CheckResult {
	_, err := runner.LimaShell(ctx, vmName, fmt.Sprintf("ping -c 1 -W 3 %s", toIP))
	name := fmt.Sprintf("Ping %s", toHost)
	if err != nil {
		return CheckResult{Name: name, Host: fromHost, Passed: false, Message: fmt.Sprintf("cannot reach %s (%s)", toHost, toIP)}
	}
	return CheckResult{Name: name, Host: fromHost, Passed: true, Message: fmt.Sprintf("reachable (%s)", toIP)}
}

func checkClusterHealth(ctx context.Context, mgr *k3s.Manager, runner *remote.Runner, vmName, host string, cfg *config.Config, fix bool) CheckResult {
	out, err := mgr.GetNodeStatus(ctx)
	if err != nil {
		return CheckResult{Name: "Cluster Health", Host: host, Passed: false, Message: "API server not reachable"}
	}

	passed := !strings.Contains(out, "NotReady") && !strings.Contains(out, "SchedulingDisabled")
	if !passed && fix {
		fmt.Printf("  [%s] 🔧 Auto-fixing Cluster Health (uncordoning nodes)...\n", host)
		// Try to uncordon all nodes to see if it fixes SchedulingDisabled
		for _, n := range cfg.Nodes {
			_, _ = runner.LimaShellSudo(ctx, vmName, fmt.Sprintf("k3s kubectl uncordon %s 2>/dev/null || true", n.Host))
		}
		out, err = mgr.GetNodeStatus(ctx)
		if err == nil && !strings.Contains(out, "NotReady") && !strings.Contains(out, "SchedulingDisabled") {
			return CheckResult{Name: "Cluster Health", Host: host, Passed: true, Message: "all nodes Ready (fixed)"}
		}
	}

	if strings.Contains(out, "NotReady") {
		return CheckResult{Name: "Cluster Health", Host: host, Passed: false, Message: "some nodes are NotReady"}
	}
	if strings.Contains(out, "SchedulingDisabled") {
		return CheckResult{Name: "Cluster Health", Host: host, Passed: false, Message: "some nodes are cordoned (SchedulingDisabled)"}
	}
	return CheckResult{Name: "Cluster Health", Host: host, Passed: true, Message: "all nodes Ready"}
}

func collectBridgedIPs(ctx context.Context, cfg *config.Config) map[string]string {
	ips := make(map[string]string)
	for _, node := range cfg.Nodes {
		runner := util.NewRunner(node)

		// Prefer Tailscale IP if it's installed
		out, err := runner.LimaShell(ctx, node.GetVMName(), "tailscale ip -4 2>/dev/null")
		tsIP := strings.TrimSpace(out)
		if err == nil && tsIP != "" {
			ips[node.Host] = tsIP
			continue
		}

		mgr := lima.NewManager(runner, node)
		ip, err := mgr.GetBridgedIP(ctx)
		if err == nil {
			ips[node.Host] = ip
		}
	}
	return ips
}
