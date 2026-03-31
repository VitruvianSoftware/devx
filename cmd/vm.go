package cmd

import "github.com/spf13/cobra"

var vmCmd = &cobra.Command{
	Use:   "vm",
	Short: "Manage the local development VM",
	Long: `Commands for provisioning, inspecting, and managing the Fedora CoreOS
Podman VM that powers your local dev environment.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
