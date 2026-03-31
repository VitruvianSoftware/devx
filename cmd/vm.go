package cmd

import (
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/provider"
)

var vmProviderFlag string

var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "Manage the local development VM",
	Long: `Commands for provisioning, inspecting, and managing your local dev environment.
Supports multiple backends via --provider (podman, docker, orbstack).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// getVMProvider resolves the --provider flag to a concrete VMProvider.
func getVMProvider() (provider.VMProvider, error) {
	return provider.Get(vmProviderFlag)
}

func init() {
	vmCmd.PersistentFlags().StringVar(&vmProviderFlag, "provider", "podman",
		"Virtualization backend to use (podman, docker, orbstack)")
}
