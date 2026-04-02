package cmd

import (
	"github.com/VitruvianSoftware/devx/internal/k8s"
	"github.com/spf13/cobra"
)

var k8sRmRuntime string

var k8sRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Stop and remove a k3s cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return k8s.Remove(k8sRmRuntime, args[0])
	},
}

func init() {
	k8sRmCmd.Flags().StringVar(&k8sRmRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	k8sCmd.AddCommand(k8sRmCmd)
}
