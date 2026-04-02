package cmd

import (
	devxmock "github.com/VitruvianSoftware/devx/internal/mock"
	"github.com/spf13/cobra"
)

var mockRestartRuntime string

var mockRestartCmd = &cobra.Command{
	Use:   "restart <name>",
	Short: "Restart a running mock server (re-fetches the OpenAPI spec on boot)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return devxmock.Restart(mockRestartRuntime, args[0])
	},
}

func init() {
	mockRestartCmd.Flags().StringVar(&mockRestartRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	mockCmd.AddCommand(mockRestartCmd)
}
