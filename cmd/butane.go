package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var butaneCmd = &cobra.Command{
	Use:                "butane [args...]",
	Short:              "Pass-through command for local Butane utility.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		pCmd := exec.Command("butane", args...)
		pCmd.Stdin = os.Stdin
		pCmd.Stdout = os.Stdout
		pCmd.Stderr = os.Stderr
		return pCmd.Run()
	},
}

func init() {
	execCmd.AddCommand(butaneCmd)
}
