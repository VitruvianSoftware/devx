package cmd

import (
	"os"
	"os/exec"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/spf13/cobra"
)

var tailscaleCmd = &cobra.Command{
	Use:                "tailscale [args...]",
	Short:              "Pass-through command for Tailscale interacting explicitly with the VM daemon.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		devName := os.Getenv("USER")
		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil {
			cfg.DevHostname = s.DevHostname
		}

		// Inject tailscale executable calls as root using VM's tailscaled container
		// e.g., devx tailscale status -> podman machine ssh VM "sudo podman exec tailscaled tailscale status"
		sshArgs := []string{
			"machine", "ssh", cfg.DevHostname,
			"sudo", "podman", "exec", "-it", "tailscaled", "tailscale",
		}
		sshArgs = append(sshArgs, args...)

		pCmd := exec.Command("podman", sshArgs...)
		pCmd.Stdin = os.Stdin
		pCmd.Stdout = os.Stdout
		pCmd.Stderr = os.Stderr
		return pCmd.Run()
	},
}

func init() {
	execCmd.AddCommand(tailscaleCmd)
}
