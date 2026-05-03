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
	"os"
	"text/tabwriter"

	"github.com/VitruvianSoftware/devx/internal/state"
	"github.com/spf13/cobra"
)

var stateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all time-travel checkpoints",
	Long:  `Lists all locally stored CRIU checkpoints created via devx state checkpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		checkpoints, err := state.ListCheckpoints()
		if err != nil {
			return err
		}

		if outputJSON {
			b, _ := json.MarshalIndent(checkpoints, "", "  ")
			fmt.Println(string(b))
			return nil
		}

		if len(checkpoints) == 0 {
			fmt.Println("No time-travel checkpoints found.")
			fmt.Println("Create one with: devx state checkpoint <name>")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tCONTAINERS\tSIZE\tCREATED")
		_, _ = fmt.Fprintln(w, "────\t──────────\t────\t───────")
		for _, cp := range checkpoints {
			_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
				cp.Name,
				cp.ContainerCount,
				humanizeBytes(cp.SizeBytes),
				cp.CreatedAt,
			)
		}
		return w.Flush()
	},
}

func init() {
	stateCmd.AddCommand(stateListCmd)
}
