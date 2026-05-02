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
	"encoding/json"
	"fmt"

	"github.com/VitruvianSoftware/devx/internal/k8s"
	"github.com/spf13/cobra"
)

var k8sListRuntime string

var k8sListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all running k3s clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime := k8sListRuntime
		clusters, err := k8s.List(runtime)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(clusters)
		}

		if len(clusters) == 0 {
			fmt.Println("No k3s clusters running. Start one with: devx k8s spawn")
			return nil
		}

		fmt.Printf("%-16s %-8s %-50s %s\n", "NAME", "PORT", "KUBECONFIG ISOLATION", "STATUS")
		fmt.Printf("%-16s %-8s %-50s %s\n", "----", "----", "--------------------", "------")
		for _, c := range clusters {
			fmt.Printf("%-16s %-8d %-50s %s\n", c.Name, c.Port, c.ConfigPath, c.Status)
		}
		return nil
	},
}

func init() {
	k8sListCmd.Flags().StringVar(&k8sListRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	k8sCmd.AddCommand(k8sListCmd)
}
