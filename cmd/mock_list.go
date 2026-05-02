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

	devxmock "github.com/VitruvianSoftware/devx/internal/mock"
	"github.com/spf13/cobra"
)

var mockListRuntime string

var mockListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all running devx mock servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime := mockListRuntime
		infos, err := devxmock.List(runtime)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(infos)
		}

		if len(infos) == 0 {
			fmt.Println("No mock servers running. Start one with: devx mock up")
			return nil
		}

		fmt.Printf("%-16s %-8s %-44s %s\n", "NAME", "PORT", "ENV VAR INJECTION", "STATUS")
		fmt.Printf("%-16s %-8s %-44s %s\n", "----", "----", "-----------------", "------")
		for _, m := range infos {
			fmt.Printf("%-16s %-8d %-44s %s\n", m.Name, m.Port, m.EnvVar, m.Status)
		}
		return nil
	},
}

func init() {
	mockListCmd.Flags().StringVar(&mockListRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	mockCmd.AddCommand(mockListCmd)
}
