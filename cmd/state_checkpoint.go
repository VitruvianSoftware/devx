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

var stateCheckpointCmd = &cobra.Command{
	Use:   "checkpoint <name>",
	Short: "Snapshot the entire running topology's RAM and volumes using CRIU",
	Long: `Creates a point-in-time time-travel checkpoint of all running devx-managed
containers (including their memory, network sockets, and volumes) using Podman's
CRIU integration.

Useful for stepping back to a clean state exactly 5 minutes before a bug occurred.

Note: Requires --provider=podman mapping. Docker Mac does not natively support CRIU.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		prov, err := getFullProvider()
		if err != nil {
			return err
		}

		if DryRun {
			fmt.Printf("[dry-run] Would snapshot all devx-managed running containers into %s\n", name)
			return nil
		}

		if !NonInteractive {
			fmt.Printf("⚠️  About to snapshot all running containers to checkpoint %q\n", name)
			fmt.Print("Continue? [y/N] ")
			var confirm string
			fmt.Scanln(&confirm) //nolint:errcheck
			if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := state.CreateCheckpoint(prov.VM.Name(), name, prov.Runtime); err != nil {
			return fmt.Errorf("checkpoint failed: %w", err)
		}

		fmt.Printf("\n✅ Checkpoint %q created successfully.\n", name)
		fmt.Printf("Restore later with: devx state restore %s\n", name)
		return nil
	},
}

func init() {
	stateCmd.AddCommand(stateCheckpointCmd)
}
