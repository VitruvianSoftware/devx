package cmd

import "github.com/spf13/cobra"

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Zero-config local Kubernetes clusters powered by k3s",
	Long: `Provision and manage lightning-fast, zero-config local Kubernetes clusters
running natively within devx via k3s. No Minikube or Kind installation required.`,
}

func init() {
	rootCmd.AddCommand(k8sCmd)
}
