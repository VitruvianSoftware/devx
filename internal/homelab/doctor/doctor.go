// Package doctor provides pre-flight and health checks for the homelab cluster.
package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/lima"
	"github.com/VitruvianSoftware/devx/internal/homelab/k3s"
	"github.com/VitruvianSoftware/devx/internal/homelab/remote"
)

// CheckResult represents the outcome of a single diagnostic check.
type CheckResult struct {
	Name    string
	Host    string
	Passed  bool
	Message string
}

// Run executes all diagnostic checks and reports the results.
func Run(ctx context.Context, cfg *config.Config) error {
	fmt.Println("🩺 Running diagnostics for cluster:", cfg.Cluster.Name)
	fmt.Println()

	var results []CheckResult
	var failed int

	for _, node := range cfg.Nodes {
		runner := remote.NewRunner(node.Host)

		// Check 1: SSH connectivity.
		results = append(results, checkSSH(ctx, runner, node.Host))

		// Check 2: Homebrew.
		results = append(results, checkBrew(ctx, runner, node.Host))

		// Check 3: Lima.
		results = append(results, checkLima(ctx, runner, node.Host))

		// Check 4: socket_vmnet.
		results = append(results, checkSocketVmnet(ctx, runner, node.Host))

		// Check 5: VM status.
		limaMgr := lima.NewManager(runner, node)
		results = append(results, checkVM(ctx, limaMgr, node.Host))

		// Check 6: Bridged IP.
		results = append(results, checkBridgedIP(ctx, limaMgr, node.Host))

		// Check 7: K3s installation.
		k3sMgr := k3s.NewManager(runner)
		results = append(results, checkK3s(ctx, k3sMgr, node.Host))
	}

	// Check 8: Cross-VM connectivity (if multiple VMs are running).
	ips := collectBridgedIPs(ctx, cfg)
	if len(ips) > 1 {
		for host, ip := range ips {
			for otherHost, otherIP := range ips {
				if host == otherHost {
					continue
				}
				runner := remote.NewRunner(host)
				limaMgr := lima.NewManager(runner, config.NodeConfig{Host: host})
				results = append(results, checkPing(ctx, limaMgr, host, otherHost, otherIP, ip))
			}
		}
	}

	// Check 9: Cluster health (from init node).
	initNode := cfg.InitNode()
	initRunner := remote.NewRunner(initNode.Host)
	initK3s := k3s.NewManager(initRunner)
	results = append(results, checkClusterHealth(ctx, initK3s, initNode.Host))

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

func checkSocketVmnet(ctx context.Context, runner *remote.Runner, host string) CheckResult {
	cmd := `if sudo test -S /opt/homebrew/var/run/socket_vmnet; then echo /opt/homebrew/var/run/socket_vmnet; elif sudo test -S /usr/local/var/run/socket_vmnet; then echo /usr/local/var/run/socket_vmnet; elif sudo test -S /var/run/socket_vmnet; then echo /var/run/socket_vmnet; else exit 1; fi`
	out, err := runner.RunShell(ctx, cmd)
	if err != nil {
		return CheckResult{Name: "socket_vmnet", Host: host, Passed: false, Message: "socket not found"}
	}
	return CheckResult{Name: "socket_vmnet", Host: host, Passed: true, Message: fmt.Sprintf("running at %s", strings.TrimSpace(out))}
}

func checkVM(ctx context.Context, mgr *lima.Manager, host string) CheckResult {
	status, err := mgr.Status(ctx)
	if err != nil {
		return CheckResult{Name: "Lima VM", Host: host, Passed: false, Message: fmt.Sprintf("error: %v", err)}
	}
	passed := status == lima.VMStatusRunning
	return CheckResult{Name: "Lima VM", Host: host, Passed: passed, Message: string(status)}
}

func checkBridgedIP(ctx context.Context, mgr *lima.Manager, host string) CheckResult {
	ip, err := mgr.GetBridgedIP(ctx)
	if err != nil {
		return CheckResult{Name: "Bridged IP", Host: host, Passed: false, Message: fmt.Sprintf("not available: %v", err)}
	}
	return CheckResult{Name: "Bridged IP", Host: host, Passed: true, Message: ip}
}

func checkK3s(ctx context.Context, mgr *k3s.Manager, host string) CheckResult {
	installed, err := mgr.IsInstalled(ctx)
	if err != nil {
		return CheckResult{Name: "K3s", Host: host, Passed: false, Message: fmt.Sprintf("error: %v", err)}
	}
	if !installed {
		return CheckResult{Name: "K3s", Host: host, Passed: false, Message: "not installed"}
	}
	return CheckResult{Name: "K3s", Host: host, Passed: true, Message: "installed"}
}

func checkPing(ctx context.Context, mgr *lima.Manager, fromHost, toHost, toIP, fromIP string) CheckResult {
	runner := remote.NewRunner(fromHost)
	_, err := runner.LimaShell(ctx, "k8s-node", fmt.Sprintf("ping -c 1 -W 3 %s", toIP))
	name := fmt.Sprintf("Ping %s", toHost)
	if err != nil {
		return CheckResult{Name: name, Host: fromHost, Passed: false, Message: fmt.Sprintf("cannot reach %s (%s)", toHost, toIP)}
	}
	return CheckResult{Name: name, Host: fromHost, Passed: true, Message: fmt.Sprintf("reachable (%s)", toIP)}
}

func checkClusterHealth(ctx context.Context, mgr *k3s.Manager, host string) CheckResult {
	out, err := mgr.GetNodeStatus(ctx)
	if err != nil {
		return CheckResult{Name: "Cluster Health", Host: host, Passed: false, Message: "API server not reachable"}
	}
	if strings.Contains(out, "NotReady") {
		return CheckResult{Name: "Cluster Health", Host: host, Passed: false, Message: "some nodes are NotReady"}
	}
	return CheckResult{Name: "Cluster Health", Host: host, Passed: true, Message: "all nodes Ready"}
}

func collectBridgedIPs(ctx context.Context, cfg *config.Config) map[string]string {
	ips := make(map[string]string)
	for _, node := range cfg.Nodes {
		runner := remote.NewRunner(node.Host)
		mgr := lima.NewManager(runner, node)
		ip, err := mgr.GetBridgedIP(ctx)
		if err == nil {
			ips[node.Host] = ip
		}
	}
	return ips
}
