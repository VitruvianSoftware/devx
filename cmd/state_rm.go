package cmd

import (
	"fmt"

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
