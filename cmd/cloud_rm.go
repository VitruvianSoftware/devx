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
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/cloud"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var cloudRmRuntime string

var cloudRmCmd = &cobra.Command{
	Use:   "rm <service>",
	Short: "Stop and remove a devx-managed cloud emulator",
	Long:  `Stops and removes the emulator container. No persistent volumes are created by emulators, so no data is kept.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runCloudRm,
}

func init() {
	cloudRmCmd.Flags().StringVar(&cloudRmRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	cloudCmd.AddCommand(cloudRmCmd)
}

func runCloudRm(_ *cobra.Command, args []string) error {
	serviceName := strings.ToLower(args[0])
	if _, ok := cloud.Registry[serviceName]; !ok {
		return fmt.Errorf("unknown service %q — supported: %s",
			serviceName, strings.Join(cloud.SupportedServices(), ", "))
	}

	runtime := cloudRmRuntime
	containerName := fmt.Sprintf("devx-cloud-%s", serviceName)

	if DryRun {
		fmt.Printf("[dry-run] Would stop and remove container %s\n", containerName)
		return nil
	}

	if !NonInteractive {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Remove %s emulator?", serviceName)).
					Description(fmt.Sprintf("This will stop and remove container '%s'.", containerName)).
					Affirmative("Yes, remove it").
					Negative("Cancel").
					Value(&confirmed),
			),
		)
		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Printf("Stopping %s...\n", containerName)
	_ = exec.Command(runtime, "stop", containerName).Run()

	fmt.Printf("Removing %s...\n", containerName)
	if err := exec.Command(runtime, "rm", "-f", containerName).Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	fmt.Printf("✅ %s emulator removed.\n", serviceName)
	return nil
}
