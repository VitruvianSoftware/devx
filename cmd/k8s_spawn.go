package cmd

import (
	"fmt"

	"github.com/VitruvianSoftware/devx/internal/k8s"
	"github.com/spf13/cobra"
)

var k8sSpawnRuntime string

var k8sSpawnCmd = &cobra.Command{
	Use:   "spawn [name]",
	Short: "Spin up a zero-config local Kubernetes cluster",
	Long: `Boot a lightning-fast k3s cluster directly within devx. 
	
Extracts an isolated, safely scoped kubeconfig directly to your ~/.kube/ directory
without risking corruption to your personal host ~/.kube/config.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime := k8sSpawnRuntime
		if runtime != "podman" && runtime != "docker" {
			return fmt.Errorf("unsupported runtime %q — use 'podman' or 'docker'", runtime)
		}

		name := "local"
		if len(args) > 0 {
			name = args[0]
		}

		cluster, err := k8s.Spawn(runtime, name)
		if err != nil {
			return err
		}

		fmt.Printf("\n✅ Cluster %q is ready!\n", cluster.Name)
		fmt.Printf("   API Server: https://127.0.0.1:%d\n", cluster.Port)
		fmt.Printf("   Kubeconfig: %s\n\n", cluster.ConfigPath)
		fmt.Println("Connect to your new cluster by running:")
		fmt.Printf("\n    export KUBECONFIG=%s\n", cluster.ConfigPath)
		fmt.Printf("    kubectl get nodes\n\n")

		return nil
	},
}

func init() {
	k8sSpawnCmd.Flags().StringVar(&k8sSpawnRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	k8sCmd.AddCommand(k8sSpawnCmd)
}
