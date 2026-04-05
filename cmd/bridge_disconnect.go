package cmd

import (
	"encoding/json"
	"fmt"

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

	if session == nil || len(session.Entries) == 0 {
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
				DryRun  bool                  `json:"dry_run"`
				Entries []bridge.SessionEntry `json:"entries_to_disconnect"`
			}
			out := dryRunOutput{
				DryRun:  true,
				Entries: session.Entries,
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
		fmt.Printf("\n  [dry-run] Would disconnect %d bridge(s) and clean up session files.\n\n", len(session.Entries))
		return nil
	}

	// Execute cleanup
	if err := bridge.ClearSession(); err != nil {
		return fmt.Errorf("cleaning session: %w", err)
	}

	if outputJSON {
		type disconnectOutput struct {
			Disconnected bool `json:"disconnected"`
			Count        int  `json:"count"`
		}
		enc, _ := json.MarshalIndent(disconnectOutput{
			Disconnected: true,
			Count:        len(session.Entries),
		}, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	fmt.Printf("\n%s Disconnected %d bridge(s). Session files cleaned.\n\n",
		tui.IconDone, len(session.Entries))

	return nil
}
