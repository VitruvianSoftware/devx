package cmd

import (
	devxmock "github.com/VitruvianSoftware/devx/internal/mock"
	"github.com/spf13/cobra"
)

var mockRmRuntime string

var mockRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Stop and remove a mock server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return devxmock.Remove(mockRmRuntime, args[0])
	},
}

func init() {
	mockRmCmd.Flags().StringVar(&mockRmRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	mockCmd.AddCommand(mockRmCmd)
}
