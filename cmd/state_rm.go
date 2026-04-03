package cmd

import (
	"fmt"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/state"
	"github.com/spf13/cobra"
)

var stateRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Delete a time-travel checkpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if DryRun {
			fmt.Printf("[dry-run] Would delete checkpoint %q\n", name)
			return nil
		}

		if !NonInteractive {
			fmt.Printf("⚠️  This will permanently delete checkpoint %q and all its archives.\n", name)
			fmt.Print("Continue? [y/N] ")
			var confirm string
			fmt.Scanln(&confirm) //nolint:errcheck
			if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := state.DeleteCheckpoint(name); err != nil {
			return err
		}

		fmt.Printf("🗑️  Checkpoint %q deleted.\n", name)
		return nil
	},
}

func init() {
	stateCmd.AddCommand(stateRmCmd)
}
