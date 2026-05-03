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
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/bridge"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var (
	interceptPort      int
	interceptLocalPort int
	interceptSteal     bool
	interceptMirror    bool
	interceptAgentImg  string
	interceptKubeconfig string
	interceptContext   string
	interceptNamespace string
)

var bridgeInterceptCmd = &cobra.Command{
	Use:   "intercept <service>",
	Short: "Intercept inbound traffic from a remote Kubernetes service",
	Long: `Routes real cluster traffic to your local machine for live debugging.

Deploys an ephemeral agent pod to the cluster, swaps the target Service's
selector to route traffic through the agent, and tunnels inbound requests
to your local application via a Yamux multiplexed connection.

The agent is self-healing: if the CLI crashes or disconnects, the agent
automatically restores the original Service selector before exiting.

Requires explicit --steal flag to acknowledge traffic redirection.
Mirror mode (--mirror) is planned for a future release.

Examples:
  # Intercept all traffic to payments-api (steal mode)
  devx bridge intercept payments-api --steal

  # Specify ports explicitly
  devx bridge intercept payments-api --steal --port 8080 --local-port 8080

  # Use a custom agent image (air-gapped clusters)
  devx bridge intercept payments-api --steal --agent-image my-reg/agent:v1

  # Dry-run: see what would happen without modifying the cluster
  devx bridge intercept payments-api --steal --dry-run

  # Machine-readable output for AI agents
  devx bridge intercept payments-api --steal --json`,
	Args: cobra.ExactArgs(1),
	RunE: runBridgeIntercept,
}

func init() {
	bridgeInterceptCmd.Flags().IntVarP(&interceptPort, "port", "p", 0,
		"Remote port to intercept (default: first port on the Service)")
	bridgeInterceptCmd.Flags().IntVar(&interceptLocalPort, "local-port", 0,
		"Local port where traffic is routed (default: same as --port)")
	bridgeInterceptCmd.Flags().BoolVar(&interceptSteal, "steal", false,
		"Full traffic redirect — original pod receives zero traffic (required)")
	bridgeInterceptCmd.Flags().BoolVar(&interceptMirror, "mirror", false,
		"Duplicate traffic only (not yet implemented — planned for 46.2b)")
	bridgeInterceptCmd.Flags().StringVar(&interceptAgentImg, "agent-image", "",
		"Override the default agent container image (for air-gapped/private registries)")
	bridgeInterceptCmd.Flags().StringVar(&interceptKubeconfig, "kubeconfig", "",
		"Path to kubeconfig file (default: ~/.kube/config)")
	bridgeInterceptCmd.Flags().StringVar(&interceptContext, "context", "",
		"Kubernetes context to use")
	bridgeInterceptCmd.Flags().StringVarP(&interceptNamespace, "namespace", "n", "",
		"Target namespace (default: from devx.yaml or 'default')")
}

func runBridgeIntercept(_ *cobra.Command, args []string) error {
	serviceName := args[0]

	// 1. Validate mode flag
	if interceptMirror {
		return fmt.Errorf("--mirror mode is not yet implemented — planned for Idea 46.2b")
	}
	if !interceptSteal {
		return fmt.Errorf("mode required: specify --steal (redirects all traffic) or --mirror (not yet available)")
	}

	// 2. Resolve kubeconfig/context/namespace from flags or devx.yaml
	kubeconfig, kubeCtx, namespace, agentImage := resolveInterceptConfig()

	if namespace == "" {
		namespace = "default"
	}

	// 3. Validate kubectl is available
	kubectlVer, err := bridge.ValidateKubectl()
	if err != nil {
		return err
	}

	// 4. Resolve kubeconfig path
	kcPath, err := bridge.ResolveKubeconfig(kubeconfig)
	if err != nil {
		return err
	}

	// 5. Validate context
	if err := bridge.ValidateContext(kcPath, kubeCtx); err != nil {
		return err
	}

	// 6. Inspect target service
	info, err := bridge.InspectService(kcPath, kubeCtx, namespace, serviceName)
	if err != nil {
		return err
	}

	// 7. Validate service is interceptable
	if err := bridge.ValidateInterceptable(info); err != nil {
		return err
	}

	// 8. Warn if mesh sidecar detected
	if info.HasMeshSidecar && !outputJSON {
		fmt.Printf("\n  ⚠️  Service mesh sidecar detected on %s — intercept may not work as expected.\n", serviceName)
		fmt.Printf("     See docs/guide/bridge.md#service-mesh\n\n")
	}

	// 9. Resolve intercept port
	targetPort := interceptPort
	if targetPort == 0 && len(info.Ports) > 0 {
		targetPort = info.Ports[0].Port
	}
	if targetPort == 0 {
		return fmt.Errorf("could not determine target port — specify --port explicitly")
	}

	localPort := interceptLocalPort
	if localPort == 0 {
		localPort = targetPort
	}

	// 10. Check for conflicts
	if err := bridge.CheckInterceptConflict(kcPath, kubeCtx, namespace, serviceName); err != nil {
		return err
	}

	// Generate session ID
	sessionID := uuid.New().String()[:8]

	// 11. Dry-run
	if DryRun {
		return printInterceptDryRun(info, kcPath, kubeCtx, kubectlVer, targetPort, localPort, agentImage, sessionID)
	}

	// 12. Display banner
	if !outputJSON {
		fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("🔀 devx bridge intercept"))
		fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("kubectl:"), kubectlVer)
		fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("config:"), kcPath)
		if kubeCtx != "" {
			fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("context:"), kubeCtx)
		}
		fmt.Printf("  %s  %s/%s\n", tui.StyleLabel.Render("target:"),
			tui.StyleMuted.Render(namespace), tui.StyleStepName.Render(serviceName))
		fmt.Printf("  %s  %s (:%d → localhost:%d)\n", tui.StyleLabel.Render("mode:"),
			tui.StyleDetailRunning.Render("steal"), targetPort, localPort)
		fmt.Println()
	}

	// 13. Deploy agent
	if !outputJSON {
		fmt.Printf("  %s Deploying agent pod...\n", tui.StyleDetailRunning.Render("●"))
	}

	agentCfg := bridge.AgentConfig{
		Kubeconfig:       kcPath,
		Context:          kubeCtx,
		Namespace:        namespace,
		TargetService:    serviceName,
		InterceptPort:    targetPort,
		ServicePorts:     info.Ports,
		OriginalSelector: info.Selector,
		AgentImage:       agentImage,
		SessionID:        sessionID,
	}

	agentInfo, err := bridge.DeployAgent(agentCfg)
	if err != nil {
		return err
	}

	if !outputJSON {
		fmt.Printf("  %s  Agent pod %s is ready\n", tui.IconDone, agentInfo.PodName)
	}

	// 14. Patch service selector
	if !outputJSON {
		fmt.Printf("  %s Patching service selector...\n", tui.StyleDetailRunning.Render("●"))
	}

	agentSelector := map[string]string{
		"devx-bridge":         "agent",
		"devx-bridge-session": sessionID,
	}

	svcState, err := bridge.PatchServiceSelector(kcPath, kubeCtx, namespace, serviceName, agentSelector, sessionID)
	if err != nil {
		// Clean up agent on failure
		_ = bridge.RemoveAgent(kcPath, kubeCtx, namespace, sessionID)
		return err
	}

	if !outputJSON {
		fmt.Printf("  %s  Service selector patched (traffic redirected)\n", tui.IconDone)
	}

	// 15. Establish Yamux tunnel
	if !outputJSON {
		fmt.Printf("  %s Establishing tunnel...\n", tui.StyleDetailRunning.Render("●"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tunnel := bridge.NewTunnel(bridge.TunnelConfig{
		Kubeconfig:  kcPath,
		Context:     kubeCtx,
		Namespace:   namespace,
		AgentPod:    agentInfo.PodName,
		ControlPort: agentInfo.ControlPort,
		LocalPort:   localPort,
	})

	if err := tunnel.Start(ctx); err != nil {
		// Restore selector and clean up agent
		_ = bridge.RestoreServiceSelector(kcPath, kubeCtx, svcState)
		_ = bridge.RemoveAgent(kcPath, kubeCtx, namespace, sessionID)
		return err
	}

	if !outputJSON {
		fmt.Printf("  %s  Tunnel established\n\n", tui.IconDone)
	}

	// 16. Persist session
	interceptEntry := bridge.InterceptEntry{
		Service:          serviceName,
		Namespace:        namespace,
		TargetPort:       targetPort,
		LocalPort:        localPort,
		Mode:             "steal",
		AgentPod:         agentInfo.PodName,
		SessionID:        sessionID,
		OriginalSelector: info.Selector,
		StartedAt:        time.Now(),
	}

	session, _ := bridge.LoadSession()
	if session == nil {
		session = &bridge.Session{
			Kubeconfig: kcPath,
			Context:    kubeCtx,
			StartedAt:  time.Now(),
		}
	}
	session.Intercepts = append(session.Intercepts, interceptEntry)
	if err := bridge.SaveSession(session); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "⚠️  Could not save session: %v\n", err)
	}

	// 17. Output
	if outputJSON {
		return printInterceptJSON(interceptEntry, agentInfo)
	}

	fmt.Printf("  %s\n", tui.StyleTitle.Render("Intercept Active"))
	fmt.Printf("    %s  %s/%s :%d → localhost:%d  %s\n",
		tui.IconDone,
		tui.StyleMuted.Render(namespace),
		tui.StyleStepName.Render(serviceName),
		targetPort, localPort,
		tui.StyleDetailRunning.Render("steal"),
	)
	fmt.Printf("\n  %s\n", tui.StyleMuted.Render("Press Ctrl+C to stop intercepting and restore the service."))
	fmt.Printf("  %s\n\n", tui.StyleMuted.Render("The agent will auto-restore the service if this process crashes."))

	// 18. Wait for shutdown
	select {
	case <-ctx.Done():
	case <-tunnel.Done():
		_, _ = fmt.Fprintf(os.Stderr, "\n⚠️  Tunnel connection lost\n")
	}

	// 19. Graceful teardown
	if !outputJSON {
		fmt.Printf("\n  %s Restoring service selector...\n", tui.StyleDetailRunning.Render("●"))
	}
	stop()
	tunnel.Stop()

	if err := bridge.RestoreServiceSelector(kcPath, kubeCtx, svcState); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "⚠️  Failed to restore selector: %v\n", err)
		_, _ = fmt.Fprintf(os.Stderr, "    The agent will auto-restore it within 30s.\n")
	} else if !outputJSON {
		fmt.Printf("  %s  Service selector restored\n", tui.IconDone)
	}

	if !outputJSON {
		fmt.Printf("  %s Removing agent...\n", tui.StyleDetailRunning.Render("●"))
	}
	_ = bridge.RemoveAgent(kcPath, kubeCtx, namespace, sessionID)
	if !outputJSON {
		fmt.Printf("  %s  Agent removed\n", tui.IconDone)
	}

	// Clean intercept from session
	removeInterceptFromSession(sessionID)

	if !outputJSON {
		fmt.Printf("\n%s Intercept stopped. Service restored to normal.\n\n", tui.IconDone)
	}

	return nil
}

// resolveInterceptConfig merges CLI flags with devx.yaml bridge config.
func resolveInterceptConfig() (kubeconfig, kubeCtx, namespace, agentImage string) {
	kubeconfig = interceptKubeconfig
	kubeCtx = interceptContext
	namespace = interceptNamespace
	agentImage = interceptAgentImg

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
			if namespace == "" {
				namespace = b.Namespace
			}
			if agentImage == "" {
				agentImage = b.AgentImage
			}
		}
	}

	if agentImage == "" {
		agentImage = bridge.AgentImageDefault
	}
	return
}

// removeInterceptFromSession removes an intercept entry from the session file.
func removeInterceptFromSession(sessionID string) {
	session, err := bridge.LoadSession()
	if err != nil || session == nil {
		return
	}

	var remaining []bridge.InterceptEntry
	for _, ic := range session.Intercepts {
		if ic.SessionID != sessionID {
			remaining = append(remaining, ic)
		}
	}
	session.Intercepts = remaining

	if len(session.Entries) == 0 && len(session.Intercepts) == 0 {
		_ = bridge.ClearSession()
	} else {
		_ = bridge.SaveSession(session)
	}
}

// printInterceptDryRun shows what would happen without modifying the cluster.
func printInterceptDryRun(info *bridge.ServiceInfo, kubeconfig, kubeCtx, kubectlVer string, targetPort, localPort int, agentImage, sessionID string) error {
	if outputJSON {
		type dryRunOutput struct {
			DryRun     bool                `json:"dry_run"`
			Kubectl    string              `json:"kubectl"`
			Kubeconfig string              `json:"kubeconfig"`
			Context    string              `json:"context"`
			Service    string              `json:"service"`
			Namespace  string              `json:"namespace"`
			Mode       string              `json:"mode"`
			TargetPort int                 `json:"target_port"`
			LocalPort  int                 `json:"local_port"`
			AgentImage string              `json:"agent_image"`
			SessionID  string              `json:"session_id"`
			Ports      []bridge.ServicePortSpec `json:"ports"`
		}
		out := dryRunOutput{
			DryRun:     true,
			Kubectl:    kubectlVer,
			Kubeconfig: kubeconfig,
			Context:    kubeCtx,
			Service:    info.Name,
			Namespace:  info.Namespace,
			Mode:       "steal",
			TargetPort: targetPort,
			LocalPort:  localPort,
			AgentImage: agentImage,
			SessionID:  sessionID,
			Ports:      info.Ports,
		}
		enc, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("🔀 devx bridge intercept [dry-run]"))
	fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("kubectl:"), kubectlVer)
	fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("config:"), kubeconfig)
	if kubeCtx != "" {
		fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("context:"), kubeCtx)
	}
	fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("image:"), agentImage)
	fmt.Println()

	fmt.Printf("  %s  %s/%s :%d → localhost:%d  %s\n",
		tui.StyleDetailRunning.Render("→"),
		tui.StyleMuted.Render(info.Namespace),
		tui.StyleStepName.Render(info.Name),
		targetPort, localPort,
		tui.StyleDetailRunning.Render("steal"),
	)

	fmt.Printf("\n  %s\n", tui.StyleLabel.Render("Service Ports (will be mirrored on agent):"))
	for _, p := range info.Ports {
		name := p.Name
		if name == "" {
			name = "<unnamed>"
		}
		fmt.Printf("    %s  %s :%d → targetPort:%s (%s)\n",
			tui.StyleMuted.Render("·"),
			name, p.Port, p.TargetPort, p.Protocol,
		)
	}

	fmt.Printf("\n  [dry-run] Would deploy agent, swap selector, and establish tunnel. No changes made.\n\n")
	return nil
}

// printInterceptJSON outputs the intercept state as JSON.
func printInterceptJSON(entry bridge.InterceptEntry, agent *bridge.AgentInfo) error {
	type jsonOutput struct {
		Service    string `json:"service"`
		Namespace  string `json:"namespace"`
		Mode       string `json:"mode"`
		TargetPort int    `json:"target_port"`
		LocalPort  int    `json:"local_port"`
		AgentPod   string `json:"agent_pod"`
		SessionID  string `json:"session_id"`
		Status     string `json:"status"`
	}
	out := jsonOutput{
		Service:    entry.Service,
		Namespace:  entry.Namespace,
		Mode:       entry.Mode,
		TargetPort: entry.TargetPort,
		LocalPort:  entry.LocalPort,
		AgentPod:   agent.PodName,
		SessionID:  agent.SessionID,
		Status:     "active",
	}
	enc, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(enc))
	return nil
}
