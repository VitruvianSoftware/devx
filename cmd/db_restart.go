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

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/spf13/cobra"
)

var dbRestartRuntime string

var dbRestartCmd = &cobra.Command{
	Use:   "restart <engine>",
	Short: "Restart a devx-managed database container (volume data is preserved)",
	Long: fmt.Sprintf(`Restart a running devx-managed database container without destroying its volume.
Useful when a container becomes unresponsive or after a host reboot.

Supported engines: %s`, strings.Join(database.SupportedEngines(), ", ")),
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		engineName := strings.ToLower(args[0])
		if _, ok := database.Registry[engineName]; !ok {
			return fmt.Errorf("unknown engine %q — supported: %s",
				engineName, strings.Join(database.SupportedEngines(), ", "))
		}

		runtime := dbRestartRuntime
		if runtime != "podman" && runtime != "docker" {
			return fmt.Errorf("unsupported runtime %q — use 'podman' or 'docker'", runtime)
		}

		containerName := fmt.Sprintf("devx-db-%s", engineName)
		fmt.Printf("🔄 Restarting %s (%s)...\n", engineName, containerName)

		if err := exec.Command(runtime, "restart", containerName).Run(); err != nil {
			return fmt.Errorf("failed to restart %s: %w", engineName, err)
		}

		fmt.Printf("✓ %s restarted successfully\n", engineName)
		return nil
	},
}

func init() {
	dbRestartCmd.Flags().StringVar(&dbRestartRuntime, "runtime", "podman",
		"Container runtime to use (podman or docker)")
	dbCmd.AddCommand(dbRestartCmd)
}
