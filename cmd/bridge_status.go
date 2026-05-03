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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/bridge"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var bridgeStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active bridge sessions",
	Long: `Displays the current bridge session, including all active port-forwards,
their health states, and the corresponding BRIDGE_* environment variables.

Examples:
  devx bridge status            # human-readable output
  devx bridge status --json     # machine-readable for AI agents`,
	RunE: runBridgeStatus,
}

func runBridgeStatus(_ *cobra.Command, _ []string) error {
	session, err := bridge.LoadSession()
	if err != nil {
		return fmt.Errorf("reading bridge session: %w", err)
	}

	if session == nil || (len(session.Entries) == 0 && len(session.Intercepts) == 0) {
		if outputJSON {
			fmt.Println(`{"active": false, "entries": [], "intercepts": []}`)
			return nil
		}
		fmt.Printf("\n  %s No active bridge session.\n", tui.StyleMuted.Render("○"))
		fmt.Printf("  %s\n\n", tui.StyleMuted.Render("Run: devx bridge connect  or  devx bridge intercept <service> --steal"))
		return nil
	}

	if outputJSON {
		type jsonEntry struct {
			Service    string `json:"service"`
			Namespace  string `json:"namespace"`
			RemotePort int    `json:"remote_port"`
			LocalPort  int    `json:"local_port"`
			State      string `json:"state"`
			URL        string `json:"url"`
			EnvVar     string `json:"env_var"`
		}
		type jsonIntercept struct {
			Service    string `json:"service"`
			Namespace  string `json:"namespace"`
			TargetPort int    `json:"target_port"`
			LocalPort  int    `json:"local_port"`
			Mode       string `json:"mode"`
			AgentPod   string `json:"agent_pod"`
			SessionID  string `json:"session_id"`
		}
		type jsonOutput struct {
			Active     bool            `json:"active"`
			Kubeconfig string          `json:"kubeconfig"`
			Context    string          `json:"context"`
			StartedAt  time.Time       `json:"started_at"`
			Entries    []jsonEntry     `json:"entries"`
			Intercepts []jsonIntercept `json:"intercepts"`
		}

		out := jsonOutput{
			Active:     true,
			Kubeconfig: session.Kubeconfig,
			Context:    session.Context,
			StartedAt:  session.StartedAt,
		}
		for _, e := range session.Entries {
			out.Entries = append(out.Entries, jsonEntry{
				Service:    e.Service,
				Namespace:  e.Namespace,
				RemotePort: e.RemotePort,
				LocalPort:  e.LocalPort,
				State:      e.State,
				URL:        fmt.Sprintf("http://127.0.0.1:%d", e.LocalPort),
				EnvVar:     bridgeEnvKey(e.Service),
			})
		}
		for _, ic := range session.Intercepts {
			out.Intercepts = append(out.Intercepts, jsonIntercept{
				Service:    ic.Service,
				Namespace:  ic.Namespace,
				TargetPort: ic.TargetPort,
				LocalPort:  ic.LocalPort,
				Mode:       ic.Mode,
				AgentPod:   ic.AgentPod,
				SessionID:  ic.SessionID,
			})
		}
		enc, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	// Human-readable output
	duration := time.Since(session.StartedAt).Truncate(time.Second)

	fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("🔗 devx bridge status"))
	fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("config:"), session.Kubeconfig)
	if session.Context != "" {
		fmt.Printf("  %s  %s\n", tui.StyleLabel.Render("context:"), session.Context)
	}
	fmt.Printf("  %s  %s (%s)\n\n", tui.StyleLabel.Render("uptime:"), duration.String(), session.StartedAt.Format("15:04:05"))

	fmt.Printf("  %s\n\n", tui.StyleTitle.Render("Active Bridges"))

	for _, e := range session.Entries {
		icon := tui.IconDone
		stateLabel := tui.StyleDetailDone.Render(e.State)
		switch e.State {
		case "failed":
			icon = tui.IconFailed
			stateLabel = tui.StyleDetailError.Render(e.State)
		case "starting":
			icon = tui.StyleDetailRunning.Render("●")
			stateLabel = tui.StyleDetailRunning.Render(e.State)
		}

		fmt.Printf("    %s  %s/%s :%d → localhost:%d  %s\n",
			icon,
			tui.StyleMuted.Render(e.Namespace),
			tui.StyleStepName.Render(e.Service),
			e.RemotePort,
			e.LocalPort,
			stateLabel,
		)
		fmt.Printf("       %s %s\n",
			tui.StyleMuted.Render("env:"),
			tui.StyleMuted.Render(fmt.Sprintf("%s=http://127.0.0.1:%d", bridgeEnvKey(e.Service), e.LocalPort)),
		)
	}

	// Display intercept sessions
	if len(session.Intercepts) > 0 {
		fmt.Printf("\n  %s\n\n", tui.StyleTitle.Render("Active Intercepts"))

		for _, ic := range session.Intercepts {
			fmt.Printf("    %s  %s/%s :%d → localhost:%d  %s  (agent: %s)\n",
				tui.IconDone,
				tui.StyleMuted.Render(ic.Namespace),
				tui.StyleStepName.Render(ic.Service),
				ic.TargetPort,
				ic.LocalPort,
				tui.StyleDetailRunning.Render(ic.Mode),
				tui.StyleMuted.Render(ic.AgentPod),
			)
		}
	}

	fmt.Printf("\n  %s  %s\n",
		tui.StyleLabel.Render("env file:"),
		tui.StyleMuted.Render("~/.devx/bridge.env"),
	)
	fmt.Printf("  %s\n\n", tui.StyleMuted.Render("Variables are auto-injected by devx shell."))

	return nil
}

// bridgeEnvKey converts a service name to its BRIDGE_* env var key.
func bridgeEnvKey(service string) string {
	upper := fmt.Sprintf("BRIDGE_%s_URL",
		strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(service)))
	return upper
}
