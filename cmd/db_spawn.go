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
	"bytes"
	"fmt"
	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/database"
)

var spawnPort int
var spawnRuntime string
var spawnProject string

var spawnCmd = &cobra.Command{
	Use:   "spawn <engine>",
	Short: "Spin up a local database with persistent storage",
	Long: fmt.Sprintf(`Provision a local database in seconds with persistent volumes.

Supported engines: %s

Data is stored in a named volume (devx-data-<engine>) so it survives
container restarts and rebuilds.`, strings.Join(database.SupportedEngines(), ", ")),
	Args: cobra.ExactArgs(1),
	RunE: runSpawn,
}

func init() {
	spawnCmd.Flags().IntVarP(&spawnPort, "port", "p", 0,
		"Host port to expose (defaults to the engine's standard port)")
	spawnCmd.Flags().StringVar(&spawnRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	spawnCmd.Flags().StringVar(&spawnProject, "project", "",
		"Namespace isolation prefix for containers and volumes (e.g., pr-42)")
	dbCmd.AddCommand(spawnCmd)
}

func runSpawn(_ *cobra.Command, args []string) error {
	engineName := strings.ToLower(args[0])
	engine, ok := database.Registry[engineName]
	if !ok {
		return fmt.Errorf("unknown engine %q — supported: %s",
			engineName, strings.Join(database.SupportedEngines(), ", "))
	}

	runtime := spawnRuntime
	if runtime != "docker" && runtime != "podman" {
		return fmt.Errorf("unsupported runtime %q — use 'podman' or 'docker'", runtime)
	}

	port := spawnPort
	if port == 0 {
		port = engine.DefaultPort
	}

	containerName := fmt.Sprintf("devx-db-%s", engineName)
	volumeName := fmt.Sprintf("devx-data-%s", engineName)
	if spawnProject != "" {
		containerName = fmt.Sprintf("devx-db-%s-%s", spawnProject, engineName)
		volumeName = fmt.Sprintf("devx-data-%s-%s", spawnProject, engineName)
	}

	// Check if already running
	checkCmd := exec.Command(runtime, "inspect", containerName, "--format", "{{.State.Running}}")
	if out, err := checkCmd.Output(); err == nil && strings.TrimSpace(string(out)) == "true" {
		fmt.Printf("✓ %s is already running on port %d\n", engine.Name, port)
		fmt.Printf("  Connection: %s\n", engine.ConnString(port))
		return nil
	}

	// Remove any stopped container with the same name
	_ = exec.Command(runtime, "rm", "-f", containerName).Run()

	var finalErr error
	for {
		fmt.Printf("🚀 Spawning %s on port %d...\n", engine.Name, port)

		// Build run args
		runArgs := []string{
			"run", "-d",
			"--name", containerName,
			"-p", fmt.Sprintf("%d:%d", port, engine.InternalPort),
			"-v", fmt.Sprintf("%s:%s", volumeName, engine.VolumePath),
			"--label", "managed-by=devx",
			"--label", fmt.Sprintf("devx-engine=%s", engineName),
			"--restart", "unless-stopped",
		}

		for k, v := range engine.Env {
			runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", k, v))
		}

		runArgs = append(runArgs, engine.Image)

		cmd := exec.Command(runtime, runArgs...)
		var stderrBuf bytes.Buffer
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

		if err := cmd.Run(); err != nil {
			if strings.Contains(stderrBuf.String(), "address already in use") || strings.Contains(stderrBuf.String(), "port is already allocated") {
				_ = exec.Command(runtime, "rm", "-f", containerName).Run()

				var retry bool
				pErr := huh.NewConfirm().
					Title(fmt.Sprintf("Port %d is already in use. Try starting on port %d instead?", port, port+1)).
					Value(&retry).
					Run()

				if pErr == nil && retry {
					port++
					continue
				}
				finalErr = devxerr.New(devxerr.CodeHostPortInUse, fmt.Sprintf("Port %d is already in use by another process", port), err)
				break
			}
			finalErr = fmt.Errorf("failed to start %s: %w", engine.Name, err)
			break
		}

		// Success
		break
	}

	if finalErr != nil {
		return finalErr
	}

	connStr := engine.ConnString(port)

	fmt.Println()
	fmt.Printf("✅ %s is running!\n\n", engine.Name)
	fmt.Printf("  Container:  %s\n", containerName)
	fmt.Printf("  Port:       %d\n", port)
	fmt.Printf("  Volume:     %s (persistent)\n", volumeName)
	fmt.Printf("  Connection: %s\n", connStr)
	fmt.Println()
	fmt.Printf("  Stop:       %s stop %s\n", runtime, containerName)
	fmt.Printf("  Logs:       %s logs -f %s\n", runtime, containerName)
	fmt.Printf("  Remove:     devx db rm %s\n", engineName)

	return nil
}
