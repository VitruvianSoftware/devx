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

	devxsync "github.com/VitruvianSoftware/devx/internal/sync"
	"github.com/VitruvianSoftware/devx/internal/tui"
	"github.com/spf13/cobra"
)

var syncListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active file sync sessions",
	Long: `Display all devx-managed Mutagen sync sessions.

Examples:
  devx sync list         # human-readable table
  devx sync list --json  # machine-readable JSON for AI agents`,
	RunE: runSyncList,
}

func init() {
	syncCmd.AddCommand(syncListCmd)
}

func runSyncList(_ *cobra.Command, _ []string) error {
	if !devxsync.IsInstalled() {
		return fmt.Errorf("mutagen is not installed. Run: devx doctor install --all")
	}

	sessions, err := devxsync.ListSessions()
	if err != nil {
		return err
	}

	if outputJSON {
		enc, _ := json.MarshalIndent(sessions, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	if len(sessions) == 0 {
		fmt.Println("No active sync sessions.")
		fmt.Println("  Run 'devx sync up' to start syncing files into containers.")
		return nil
	}

	fmt.Println()
	fmt.Println(tui.StyleTitle.Render("⚡ Active Sync Sessions"))
	fmt.Println()

	for _, s := range sessions {
		statusStyle := tui.StyleDetailDone
		icon := tui.IconDone
		if s.Status != "" && s.Status != "Watching for changes" {
			statusStyle = tui.StyleDetailRunning
			icon = "⏳"
		}

		fmt.Printf("  %s  %-28s %s\n", icon, s.Name, statusStyle.Render(s.Status))
		fmt.Printf("       %s → %s\n", tui.StyleMuted.Render(s.Source), tui.StyleMuted.Render(s.Dest))
	}

	fmt.Printf("\n  %d session(s) active.\n\n", len(sessions))
	return nil
}
