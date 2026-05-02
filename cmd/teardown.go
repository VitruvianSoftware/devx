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

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/config"
)

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Stop and remove the dev VM (destructive)",
	RunE:  runTeardown,
}

var forceFlag bool

func init() {
	teardownCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Skip confirmation prompt")
	vmCmd.AddCommand(teardownCmd)
}

func runTeardown(_ *cobra.Command, _ []string) error {
	vm, err := getVMProvider()
	if err != nil {
		return err
	}

	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, "", "", "")

	if DryRun {
		fmt.Printf("DRY RUN: Would completely destroy the VM '%s' (%s)\n", cfg.DevHostname, vm.Name())
		fmt.Println("DRY RUN: Tailscale re-authentication will be required on next setup.")
		return nil
	}

	if !forceFlag && !NonInteractive {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Destroy %q?", cfg.DevHostname)).
					Description(fmt.Sprintf("This will stop and permanently delete the VM (%s) and all its data.\nTailscale re-authentication will be required on next setup.", vm.Name())).
					Affirmative("Yes, destroy it").
					Negative("Cancel").
					Value(&confirmed),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Teardown cancelled.")
			return nil
		}
	}

	fmt.Printf("Stopping %s (%s)...\n", cfg.DevHostname, vm.Name())
	_ = vm.StopAll()

	fmt.Printf("Removing %s...\n", cfg.DevHostname)
	_ = vm.Remove(cfg.DevHostname)

	fmt.Println("✓ VM removed.")
	return nil
}
