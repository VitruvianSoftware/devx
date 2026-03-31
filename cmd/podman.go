package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var podmanCmd = &cobra.Command{
	Use:                "podman [args...]",
	Short:              "Pass-through command for local Podman executing against the VM.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Run native podman with whatever args the user gave
		pCmd := exec.Command("podman", args...)
		pCmd.Stdin = os.Stdin
		pCmd.Stdout = os.Stdout
		pCmd.Stderr = os.Stderr
		return pCmd.Run()
	},
}

func init() {
	execCmd.AddCommand(podmanCmd)
}
