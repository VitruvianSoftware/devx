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

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/bridge"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var (
	bridgeKubeconfig string
	bridgeContext    string
	bridgeNamespace  string
	bridgeTargets    []string // ad-hoc targets: "service:port" or "service:port:localport"
)

var bridgeConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Establish outbound bridge to remote cluster services",
	Long: `Connects your local environment to remote Kubernetes services by
establishing kubectl port-forward tunnels for each target service.

Generates BRIDGE_<SERVICE>_URL environment variables in ~/.devx/bridge.env
that are automatically injected into devx shell.

Configuration sources (in priority order):
  1. CLI flags (--target, --context, --namespace)
  2. devx.yaml bridge section

Examples:
  # Use devx.yaml configuration
  devx bridge connect

  # Ad-hoc with CLI flags
  devx bridge connect --context staging -n default -t payments-api:8080

  # Dry-run: see what would be bridged
  devx bridge connect --dry-run

  # Machine-readable output for AI agents
  devx bridge connect --json`,
	RunE: runBridgeConnect,
}

func init() {
	bridgeConnectCmd.Flags().StringVar(&bridgeKubeconfig, "kubeconfig", "",
		"Path to kubeconfig file (default: ~/.kube/config)")
	bridgeConnectCmd.Flags().StringVar(&bridgeContext, "context", "",
		"Kubernetes context to use")
	bridgeConnectCmd.Flags().StringVarP(&bridgeNamespace, "namespace", "n", "",
		"Default namespace for target services")
	bridgeConnectCmd.Flags().StringArrayVarP(&bridgeTargets, "target", "t", nil,
		"Ad-hoc target: service:port or service:port:localport (repeatable)")
}

// bridgeTarget is the resolved representation of a service to bridge.
type bridgeTarget struct {
	Service   string `json:"service"`
	Namespace string `json:"namespace"`
	Port      int    `json:"port"`
	LocalPort int    `json:"local_port"`
}

func runBridgeConnect(_ *cobra.Command, _ []string) error {
	// 1. Resolve targets from CLI flags and/or devx.yaml
	targets, kubeconfig, kubeCtx, err := resolveBridgeConfig()
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		return fmt.Errorf("no bridge targets specified — use --target flag or add a 'bridge:' section to devx.yaml")
	}

	// 2. Validate kubectl is available
	kubectlVer, err := bridge.ValidateKubectl()
	if err != nil {
		return err
	}

	// 3. Resolve kubeconfig path
	kcPath, err := bridge.ResolveKubeconfig(kubeconfig)
	if err != nil {
		return err
	}

	// 4. Validate context/cluster access
	if err := bridge.ValidateContext(kcPath, kubeCtx); err != nil {
		return err
	}

	// 5. Resolve local ports for each target
	forwards := make([]*bridge.PortForward, 0, len(targets))
	for _, t := range targets {
		pf := bridge.NewPortForward(kcPath, kubeCtx, t.Namespace, t.Service, t.Port, t.LocalPort)
		if warning, err := pf.ResolveLocalPort(); err != nil {
			return err
		} else if warning != "" {
			fmt.Println(warning)
		}
		forwards = append(forwards, pf)
	}

	// 6. Dry-run: show what would be bridged
	if DryRun {
		return printBridgeDryRun(forwards, kcPath, kubeCtx, kubectlVer)
	}

	// 7. Validate each service exists
	if !NonInteractive {
		fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("🔗 devx bridge connect"))
		fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("kubectl:"), kubectlVer)
		fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("config:"), kcPath)
		if kubeCtx != "" {
			fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("context:"), kubeCtx)
		}
		fmt.Println()
	}

	for _, pf := range forwards {
		if err := bridge.ValidateService(kcPath, kubeCtx, pf.Namespace, pf.Service); err != nil {
			return err
		}
	}

	// 8. Build session entries and start port-forwards
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	session := &bridge.Session{
		Kubeconfig: kcPath,
		Context:    kubeCtx,
		StartedAt:  time.Now(),
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(forwards))

	for _, pf := range forwards {
		entry := bridge.SessionEntry{
			Service:    pf.Service,
			Namespace:  pf.Namespace,
			RemotePort: pf.RemotePort,
			LocalPort:  pf.LocalPort,
			State:      "starting",
			StartedAt:  time.Now(),
		}
		session.Entries = append(session.Entries, entry)

		wg.Add(1)
		go func(pf *bridge.PortForward) {
			defer wg.Done()
			if err := pf.Start(ctx); err != nil {
				errCh <- err
			}
		}(pf)
	}

	// 9. Persist session and generate env file
	if err := bridge.SaveSession(session); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not save session: %v\n", err)
	}
	if err := bridge.GenerateEnvFile(session.Entries); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not generate bridge.env: %v\n", err)
	}

	// 10. Display active bridges
	if outputJSON {
		return printBridgeJSON(session, forwards)
	}

	printBridgeStatus(forwards)
	fmt.Printf("\n  %s\n", tui.StyleMuted.Render("Press Ctrl+C to disconnect all bridges."))
	fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("env file:"),
		tui.StyleMuted.Render("~/.devx/bridge.env (auto-injected by devx shell)"))
	fmt.Println()

	// 11. Wait for shutdown signal or fatal error
	select {
	case <-ctx.Done():
		// Ctrl+C received
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "\n❌ Bridge error: %v\n", err)
	}

	// 12. Graceful teardown
	stop()
	for _, pf := range forwards {
		pf.Stop()
	}
	wg.Wait()

	// Clean up session files
	_ = bridge.ClearSession()

	if !outputJSON {
		fmt.Printf("\n%s All bridges disconnected.\n", tui.IconDone)
	}

	return nil
}

// resolveBridgeConfig merges CLI flags with devx.yaml bridge config.
func resolveBridgeConfig() ([]bridgeTarget, string, string, error) {
	var targets []bridgeTarget
	kubeconfig := bridgeKubeconfig
	kubeCtx := bridgeContext
	defaultNS := bridgeNamespace

	// Read devx.yaml if it exists
	if yamlPath, err := findDevxConfig(); err == nil {
		cfg, err := resolveConfig(yamlPath, "")
		if err == nil && cfg.Bridge != nil {
			b := cfg.Bridge
			if kubeconfig == "" {
				kubeconfig = b.Kubeconfig
			}
			if kubeCtx == "" {
				kubeCtx = b.Context
			}
			if defaultNS == "" {
				defaultNS = b.Namespace
			}

			for _, t := range b.Targets {
				ns := t.Namespace
				if ns == "" {
					ns = defaultNS
				}
				if ns == "" {
					ns = "default"
				}
				targets = append(targets, bridgeTarget{
					Service:   t.Service,
					Namespace: ns,
					Port:      t.Port,
					LocalPort: t.LocalPort,
				})
			}
		}
	}

	// CLI --target flags override/append
	for _, raw := range bridgeTargets {
		t, err := parseBridgeTarget(raw, defaultNS)
		if err != nil {
			return nil, "", "", err
		}
		targets = append(targets, t)
	}

	if defaultNS == "" {
		defaultNS = "default"
	}

	// Ensure all targets have a namespace
	for i := range targets {
		if targets[i].Namespace == "" {
			targets[i].Namespace = defaultNS
		}
	}

	return targets, kubeconfig, kubeCtx, nil
}

// parseBridgeTarget parses a CLI target string: "service:port" or "service:port:localport".
func parseBridgeTarget(raw, defaultNS string) (bridgeTarget, error) {
	parts := strings.Split(raw, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return bridgeTarget{}, fmt.Errorf("invalid target %q — expected format: service:port or service:port:localport", raw)
	}

	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return bridgeTarget{}, fmt.Errorf("invalid port in target %q: %w", raw, err)
	}

	t := bridgeTarget{
		Service:   parts[0],
		Namespace: defaultNS,
		Port:      port,
	}

	if len(parts) == 3 {
		lp, err := strconv.Atoi(parts[2])
		if err != nil {
			return bridgeTarget{}, fmt.Errorf("invalid local port in target %q: %w", raw, err)
		}
		t.LocalPort = lp
	}

	return t, nil
}

// printBridgeDryRun shows what would be bridged without connecting.
func printBridgeDryRun(forwards []*bridge.PortForward, kubeconfig, kubeCtx, kubectlVer string) error {
	if outputJSON {
		type dryRunOutput struct {
			DryRun     bool               `json:"dry_run"`
			Kubectl    string             `json:"kubectl"`
			Kubeconfig string             `json:"kubeconfig"`
			Context    string             `json:"context"`
			Forwards   []bridge.SessionEntry `json:"forwards"`
		}
		out := dryRunOutput{
			DryRun:     true,
			Kubectl:    kubectlVer,
			Kubeconfig: kubeconfig,
			Context:    kubeCtx,
		}
		for _, pf := range forwards {
			out.Forwards = append(out.Forwards, bridge.SessionEntry{
				Service:    pf.Service,
				Namespace:  pf.Namespace,
				RemotePort: pf.RemotePort,
				LocalPort:  pf.LocalPort,
				State:      "planned",
			})
		}
		enc, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("🔗 devx bridge connect [dry-run]"))
	fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("kubectl:"), kubectlVer)
	fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("config:"), kubeconfig)
	if kubeCtx != "" {
		fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("context:"), kubeCtx)
	}
	fmt.Println()

	for _, pf := range forwards {
		fmt.Printf("  %s  %s/%s :%d → localhost:%d\n",
			tui.StyleDetailRunning.Render("→"),
			tui.StyleMuted.Render(pf.Namespace),
			tui.StyleStepName.Render(pf.Service),
			pf.RemotePort,
			pf.LocalPort,
		)
	}
	fmt.Printf("\n  [dry-run] Would establish %d port-forward(s). No connections made.\n\n", len(forwards))
	return nil
}

// printBridgeStatus displays active bridges in the TUI.
func printBridgeStatus(forwards []*bridge.PortForward) {
	fmt.Printf("  %s\n", tui.StyleTitle.Render("Active Bridges"))
	fmt.Println()
	for _, pf := range forwards {
		state := pf.State()
		icon := tui.StyleDetailRunning.Render("●")
		stateLabel := tui.StyleDetailRunning.Render("connecting")
		switch state {
		case bridge.StateHealthy:
			icon = tui.IconDone
			stateLabel = tui.StyleDetailDone.Render("healthy")
		case bridge.StateFailed:
			icon = tui.IconFailed
			stateLabel = tui.StyleDetailError.Render("failed")
		case bridge.StateStopped:
			icon = tui.StyleMuted.Render("○")
			stateLabel = tui.StyleMuted.Render("stopped")
		}

		fmt.Printf("    %s  %s/%s :%d → localhost:%d  %s\n",
			icon,
			tui.StyleMuted.Render(pf.Namespace),
			tui.StyleStepName.Render(pf.Service),
			pf.RemotePort,
			pf.LocalPort,
			stateLabel,
		)
	}
}

// printBridgeJSON outputs the session state as JSON for AI agents.
func printBridgeJSON(session *bridge.Session, forwards []*bridge.PortForward) error {
	type jsonEntry struct {
		Service    string `json:"service"`
		Namespace  string `json:"namespace"`
		RemotePort int    `json:"remote_port"`
		LocalPort  int    `json:"local_port"`
		State      string `json:"state"`
		URL        string `json:"url"`
	}
	type jsonOutput struct {
		Kubeconfig string      `json:"kubeconfig"`
		Context    string      `json:"context"`
		Entries    []jsonEntry `json:"entries"`
	}

	out := jsonOutput{
		Kubeconfig: session.Kubeconfig,
		Context:    session.Context,
	}

	for _, pf := range forwards {
		out.Entries = append(out.Entries, jsonEntry{
			Service:    pf.Service,
			Namespace:  pf.Namespace,
			RemotePort: pf.RemotePort,
			LocalPort:  pf.LocalPort,
			State:      pf.State().String(),
			URL:        fmt.Sprintf("http://127.0.0.1:%d", pf.LocalPort),
		})
	}

	enc, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(enc))
	return nil
}
