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
