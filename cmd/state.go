package cmd

import (
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "Manage topological state (diagnostic dumps, checkpoints, and restores)",
	Long: `The state command hierarchy manages the macro state of the entire devx environment.
	
You can generate full diagnostic dumps containing crashes, config, and topology.
You can also checkpoint and restore the complete system snapshot using CRIU (podman only).`,
}

func init() {
	rootCmd.AddCommand(stateCmd)
}
