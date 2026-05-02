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
	"fmt"

	devxsync "github.com/VitruvianSoftware/devx/internal/sync"
	"github.com/spf13/cobra"
)

var syncRmCmd = &cobra.Command{
	Use:   "rm [names...]",
	Short: "Terminate file sync sessions",
	Long: `Terminate devx-managed Mutagen sync sessions.

If one or more service names are provided, only those sessions are terminated.
If no names are given, all devx-managed sync sessions are terminated.

Examples:
  devx sync rm           # terminate all devx sync sessions
  devx sync rm api       # terminate only the api sync session
  devx sync rm --dry-run # preview what would be terminated`,
	RunE: runSyncRm,
}

func init() {
	syncCmd.AddCommand(syncRmCmd)
}

func runSyncRm(_ *cobra.Command, args []string) error {
	if !devxsync.IsInstalled() {
		return fmt.Errorf("mutagen is not installed. Run: devx doctor install --all")
	}

	if DryRun {
		if len(args) == 0 {
			fmt.Println("[dry-run] would terminate all devx-managed sync sessions")
		} else {
			for _, name := range args {
				fmt.Printf("[dry-run] would terminate session: devx-sync-%s\n", name)
			}
		}
		return nil
	}

	if len(args) == 0 {
		// Terminate all devx-managed sessions
		sessions, err := devxsync.ListSessions()
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			fmt.Println("No active sync sessions found.")
			return nil
		}

		if err := devxsync.TerminateAll(); err != nil {
			return fmt.Errorf("failed to terminate sync sessions: %w", err)
		}
		fmt.Printf("✅ Terminated %d sync session(s).\n", len(sessions))
		return nil
	}

	// Terminate specific sessions
	for _, name := range args {
		fmt.Printf("  Terminating devx-sync-%s...\n", name)
		if err := devxsync.TerminateSession(name); err != nil {
			fmt.Printf("  ⚠️  Failed to terminate devx-sync-%s: %v\n", name, err)
		} else {
			fmt.Printf("  ✅ devx-sync-%s terminated.\n", name)
		}
	}
	return nil
}
