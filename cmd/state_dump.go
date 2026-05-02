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
	"os"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/state"
	"github.com/VitruvianSoftware/devx/internal/tailscale"
	"github.com/spf13/cobra"
)

var stateDumpFile string

var stateDumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Snapshot topology, dependencies, and crashing container logs into a Shareable Diagnostic Dump",
	Long: `Outputs a meticulously structured, redact-safe snapshot of the running environment.
Useful for sharing context in bug trackers or 'it doesn't work on my machine' discussions.
Generates output in Markdown to stdout by default, or structured JSON.`,
	RunE: runStateDump,
}

func runStateDump(_ *cobra.Command, _ []string) error {
	prov, err := getFullProvider()
	if err != nil {
		return err
	}
	vm := prov.VM
	rt := prov.Runtime

	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, "", "", "")

	vmState := "not created"
	if info, err := vm.Inspect(cfg.DevHostname); err == nil {
		vmState = info.State
	}

	tsStatus := "vm not running"
	if vm.IsRunning(cfg.DevHostname) {
		sshFn := func(machine, command string) (string, error) {
			return vm.SSH(machine, command)
		}
		tsStatus = tailscale.StatusWithSSH(cfg.DevHostname, rt.Name(), sshFn)
	}

	report, err := state.GenerateDump(cfg, prov, vmState, tsStatus)
	if err != nil {
		return fmt.Errorf("failed to generate state dump: %w", err)
	}

	var output string
	if outputJSON {
		output = state.GenerateJSON(report)
	} else {
		output = state.GenerateMarkdown(report)
	}

	if stateDumpFile != "" {
		if err := os.WriteFile(stateDumpFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("could not write dump to file: %w", err)
		}
		fmt.Printf("✅ Diagnostic state dump written to %s\n", stateDumpFile)
		return nil
	}

	fmt.Println(output)
	return nil
}

func init() {
	stateDumpCmd.Flags().StringVarP(&stateDumpFile, "file", "f", "", "Output directly to a file (e.g. /tmp/dump.md)")
	stateCmd.AddCommand(stateDumpCmd)
}
