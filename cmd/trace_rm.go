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

	"github.com/VitruvianSoftware/devx/internal/telemetry"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var traceRmRuntime string

var traceRmCmd = &cobra.Command{
	Use:   "rm <engine>",
	Short: "Stop and remove a devx-managed telemetry backend",
	Long: `Stops and removes the telemetry backend container.

If the backend was started without --persist, all trace data will be lost.
If --persist was used, data in ~/.devx/telemetry/<engine>/ is kept on disk.`,
	Args: cobra.ExactArgs(1),
	RunE: runTraceRm,
}

func init() {
	traceRmCmd.Flags().StringVar(&traceRmRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	traceCmd.AddCommand(traceRmCmd)
}

func runTraceRm(_ *cobra.Command, args []string) error {
	engineName := telemetry.Engine(strings.ToLower(args[0]))
	containerName := telemetry.ContainerName(engineName)
	runtime := traceRmRuntime

	if DryRun {
		fmt.Printf("[dry-run] Would stop and remove container %s\n", containerName)
		return nil
	}

	if !NonInteractive {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Remove %s telemetry backend?", engineName)).
					Description(fmt.Sprintf("This will stop and remove container '%s'.\nEphemeral (non-persisted) trace data will be lost.", containerName)).
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

	fmt.Printf("✅ %s telemetry backend removed.\n", engineName)
	fmt.Println("  Persisted data (if any) remains in ~/.devx/telemetry/" + string(engineName))
	return nil
}
