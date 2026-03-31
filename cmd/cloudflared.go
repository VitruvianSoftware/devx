package cmd

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var cloudflaredCmd = &cobra.Command{
	Use:                "cloudflared [args...]",
	Short:              "Pass-through command for local Cloudflared utility.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		pCmd := exec.Command("cloudflared", args...)
		pCmd.Stdin = os.Stdin
		pCmd.Stdout = os.Stdout
		pCmd.Stderr = os.Stderr
		return pCmd.Run()
	},
}

func init() {
	execCmd.AddCommand(cloudflaredCmd)
}
