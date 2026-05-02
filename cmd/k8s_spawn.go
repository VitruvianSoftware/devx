// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
