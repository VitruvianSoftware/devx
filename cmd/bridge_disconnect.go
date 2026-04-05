package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/bridge"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var bridgeDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Tear down all active bridge sessions",
	Long: `Stops all active kubectl port-forward processes, removes the bridge
session file (~/.devx/bridge.json), and cleans up the bridge env file
(~/.devx/bridge.env).

Examples:
  devx bridge disconnect          # interactive confirmation
  devx bridge disconnect -y       # auto-confirm
  devx bridge disconnect --dry-run # show what would be cleaned`,
	RunE: runBridgeDisconnect,
}

func runBridgeDisconnect(_ *cobra.Command, _ []string) error {
	session, err := bridge.LoadSession()
	if err != nil {
		return fmt.Errorf("reading bridge session: %w", err)
	}

	hasEntries := session != nil && len(session.Entries) > 0
	hasIntercepts := session != nil && len(session.Intercepts) > 0

	if !hasEntries && !hasIntercepts {
		if outputJSON {
			fmt.Println(`{"disconnected": false, "reason": "no active session"}`)
			return nil
		}
		fmt.Printf("\n  %s No active bridge session to disconnect.\n\n", tui.StyleMuted.Render("○"))
		return nil
	}

	if DryRun {
		if outputJSON {
			type dryRunOutput struct {
				DryRun     bool                  `json:"dry_run"`
				Entries    []bridge.SessionEntry  `json:"entries_to_disconnect"`
				Intercepts []bridge.InterceptEntry `json:"intercepts_to_disconnect"`
			}
			out := dryRunOutput{
				DryRun:     true,
				Entries:    session.Entries,
				Intercepts: session.Intercepts,
			}
			enc, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(enc))
			return nil
		}

		fmt.Printf("\n%s\n\n", tui.StyleTitle.Render("🔗 devx bridge disconnect [dry-run]"))
		for _, e := range session.Entries {
			fmt.Printf("  %s  Would stop: %s/%s :%d → localhost:%d\n",
				tui.StyleDetailRunning.Render("→"),
				tui.StyleMuted.Render(e.Namespace),
				tui.StyleStepName.Render(e.Service),
				e.RemotePort,
				e.LocalPort,
			)
		}
		for _, ic := range session.Intercepts {
			fmt.Printf("  %s  Would stop intercept: %s/%s :%d → localhost:%d (%s)\n",
				tui.StyleDetailRunning.Render("→"),
				tui.StyleMuted.Render(ic.Namespace),
				tui.StyleStepName.Render(ic.Service),
				ic.TargetPort,
				ic.LocalPort,
				ic.Mode,
			)
		}
		total := len(session.Entries) + len(session.Intercepts)
		fmt.Printf("\n  [dry-run] Would disconnect %d session(s) and clean up.\n\n", total)
		return nil
	}

	// Filter out DAG-managed entries (Idea 46.3: owned by devx up)
	var dagEntries []bridge.SessionEntry
	var standaloneEntries []bridge.SessionEntry
	for _, e := range session.Entries {
		if e.Origin == "dag" {
			if !outputJSON {
				fmt.Printf("  ⚠️  Skipping DAG-managed bridge %q — managed by 'devx up'. Stop devx up to teardown.\n", e.Service)
			}
			dagEntries = append(dagEntries, e)
			continue
		}
		standaloneEntries = append(standaloneEntries, e)
	}

	var dagIntercepts []bridge.InterceptEntry
	var standaloneIntercepts []bridge.InterceptEntry
	for _, ic := range session.Intercepts {
		if ic.Origin == "dag" {
			if !outputJSON {
				fmt.Printf("  ⚠️  Skipping DAG-managed intercept %q — managed by 'devx up'. Stop devx up to teardown.\n", ic.Service)
			}
			dagIntercepts = append(dagIntercepts, ic)
			continue
		}
		standaloneIntercepts = append(standaloneIntercepts, ic)
	}

	// Tear down standalone intercepts first (restore selectors + remove agents)
	for _, ic := range standaloneIntercepts {
		if !outputJSON {
			fmt.Printf("  %s Restoring %s/%s selector...\n",
				tui.StyleDetailRunning.Render("●"),
				ic.Namespace, ic.Service)
		}

		svcState := &bridge.ServiceState{
			Name:             ic.Service,
			Namespace:        ic.Namespace,
			OriginalSelector: ic.OriginalSelector,
		}
		if err := bridge.RestoreServiceSelector(session.Kubeconfig, session.Context, svcState); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to restore %s selector: %v\n", ic.Service, err)
		}
		_ = bridge.RemoveAgent(session.Kubeconfig, session.Context, ic.Namespace, ic.SessionID)

		if !outputJSON {
			fmt.Printf("  %s  %s/%s restored and agent removed\n", tui.IconDone, ic.Namespace, ic.Service)
		}
	}

	// Clean up session files or rewrite safely preserving DAG entries
	if len(dagEntries) == 0 && len(dagIntercepts) == 0 {
		if err := bridge.ClearSession(); err != nil {
			return fmt.Errorf("cleaning session: %w", err)
		}
	} else {
		session.Entries = dagEntries
		session.Intercepts = dagIntercepts
		if err := bridge.SaveSession(session); err != nil {
			return fmt.Errorf("saving session: %w", err)
		}
	}

	total := len(standaloneEntries) + len(standaloneIntercepts)

	if outputJSON {
		type disconnectOutput struct {
			Disconnected    bool `json:"disconnected"`
			BridgeCount     int  `json:"bridge_count"`
			InterceptCount  int  `json:"intercept_count"`
		}
		enc, _ := json.MarshalIndent(disconnectOutput{
			Disconnected:   true,
			BridgeCount:    len(session.Entries),
			InterceptCount: len(session.Intercepts),
		}, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	fmt.Printf("\n%s Disconnected %d session(s). All resources cleaned.\n\n",
		tui.IconDone, total)

	return nil
}
