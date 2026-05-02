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
		vm, err := getVMProvider()
		if err != nil {
			return err
		}

		// Load env safely to get the VM name
		devName := os.Getenv("USER")
		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil {
			cfg.DevHostname = s.DevHostname
		}

		// For podman, use native 'podman machine ssh'.
		// For lima/colima, drop into their respective VM shells.
		// For docker/orbstack, drop into the VM shell.
		if vm.Name() == "podman" {
			sshArgs := append([]string{"machine", "ssh", cfg.DevHostname}, args...)
			pCmd := exec.Command("podman", sshArgs...)
			pCmd.Stdin = os.Stdin
			pCmd.Stdout = os.Stdout
			pCmd.Stderr = os.Stderr
			return pCmd.Run()
		}

		if vm.Name() == "lima" {
			sshArgs := append([]string{"shell", cfg.DevHostname}, args...)
			pCmd := exec.Command("limactl", sshArgs...)
			pCmd.Stdin = os.Stdin
			pCmd.Stdout = os.Stdout
			pCmd.Stderr = os.Stderr
			return pCmd.Run()
		}

		if vm.Name() == "colima" {
			sshArgs := append([]string{"ssh", "--"}, args...)
			pCmd := exec.Command("colima", sshArgs...)
			pCmd.Stdin = os.Stdin
			pCmd.Stdout = os.Stdout
			pCmd.Stderr = os.Stderr
			return pCmd.Run()
		}

		// Docker / OrbStack: try orb first, then docker run
		if orbPath, lookErr := exec.LookPath("orb"); lookErr == nil && orbPath != "" {
			orbArgs := append([]string{"sh"}, args...)
			pCmd := exec.Command("orb", orbArgs...)
			pCmd.Stdin = os.Stdin
			pCmd.Stdout = os.Stdout
			pCmd.Stderr = os.Stderr
			return pCmd.Run()
		}

		// Fallback: privileged nsenter into Docker Desktop VM
		dockerArgs := []string{"run", "--rm", "-it", "--privileged", "--pid=host",
			"alpine:latest", "nsenter", "-t", "1", "-m", "-u", "-n", "-i", "sh"}
		pCmd := exec.Command("docker", dockerArgs...)
		pCmd.Stdin = os.Stdin
		pCmd.Stdout = os.Stdout
		pCmd.Stderr = os.Stderr
		return pCmd.Run()
	},
}

func init() {
	vmCmd.AddCommand(sshCmd)
}
