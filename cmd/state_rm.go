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
	"strings"

	"github.com/VitruvianSoftware/devx/internal/state"
	"github.com/spf13/cobra"
)

var stateRmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Delete a time-travel checkpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		if DryRun {
			fmt.Printf("[dry-run] Would delete checkpoint %q\n", name)
			return nil
		}

		if !NonInteractive {
			fmt.Printf("⚠️  This will permanently delete checkpoint %q and all its archives.\n", name)
			fmt.Print("Continue? [y/N] ")
			var confirm string
			fmt.Scanln(&confirm) //nolint:errcheck
			if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := state.DeleteCheckpoint(name); err != nil {
			return err
		}

		fmt.Printf("🗑️  Checkpoint %q deleted.\n", name)
		return nil
	},
}

func init() {
	stateCmd.AddCommand(stateRmCmd)
}
