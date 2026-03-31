package cmd

import (
	"os"
	"os/exec"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:                "ssh [args...]",
	Short:              "Drop into an SSH shell inside the VM.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load env safely to get the VM name
		devName := os.Getenv("USER")
		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil {
			cfg.DevHostname = s.DevHostname
		}

		sshArgs := append([]string{"machine", "ssh", cfg.DevHostname}, args...)
		pCmd := exec.Command("podman", sshArgs...)
		pCmd.Stdin = os.Stdin
		pCmd.Stdout = os.Stdout
		pCmd.Stderr = os.Stderr
		return pCmd.Run()
	},
}

func init() {
	vmCmd.AddCommand(sshCmd)
}
