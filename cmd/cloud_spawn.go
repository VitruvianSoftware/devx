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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/cloud"
	"github.com/spf13/cobra"
)

var cloudSpawnPort int
var cloudSpawnRuntime string

var cloudSpawnCmd = &cobra.Command{
	Use:   "spawn <service>",
	Short: "Spawn a local GCP service emulator",
	Long: fmt.Sprintf(`Provision a local GCP cloud service emulator in seconds.

Supported services: %s

Each emulator runs as a named container inside the devx VM. The endpoint
URL and the environment variable name you need are printed on stdout — copy
them directly into your .env, or use 'devx shell' to have them injected
automatically into your dev container.`, strings.Join(cloud.SupportedServices(), ", ")),
	Args: cobra.ExactArgs(1),
	RunE: runCloudSpawn,
}

func init() {
	cloudSpawnCmd.Flags().IntVarP(&cloudSpawnPort, "port", "p", 0,
		"Host port to expose (defaults to the emulator's standard port)")
	cloudSpawnCmd.Flags().StringVar(&cloudSpawnRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	cloudCmd.AddCommand(cloudSpawnCmd)
}

func runCloudSpawn(_ *cobra.Command, args []string) error {
	serviceName := strings.ToLower(args[0])
	emulator, ok := cloud.Registry[serviceName]
	if !ok {
		return fmt.Errorf("unknown service %q — supported: %s",
			serviceName, strings.Join(cloud.SupportedServices(), ", "))
	}

	runtime := cloudSpawnRuntime
	if runtime != "docker" && runtime != "podman" {
		return fmt.Errorf("unsupported runtime %q — use 'podman' or 'docker'", runtime)
	}

	port := cloudSpawnPort
	if port == 0 {
		port = emulator.DefaultPort
	}

	containerName := fmt.Sprintf("devx-cloud-%s", serviceName)
	hostPort := fmt.Sprintf("localhost:%d", port)

	// Check if already running
	checkOut, err := exec.Command(runtime, "inspect", containerName, "--format", "{{.State.Running}}").Output()
	if err == nil && strings.TrimSpace(string(checkOut)) == "true" {
		fmt.Printf("✓ %s emulator is already running on port %d\n", emulator.Name, port)
		printCloudEnvVars(emulator, hostPort)
		return nil
	}

	// Remove any stopped container with the same name
	_ = exec.Command(runtime, "rm", "-f", containerName).Run()

	fmt.Printf("🚀 Spawning %s emulator on port %d...\n", emulator.Name, port)

	runArgs := buildGCSArgs(serviceName, runtime, containerName, port, emulator)

	cmd := exec.Command(runtime, runArgs...)
	var stderrBuf bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderrBuf.String(), "address already in use") ||
			strings.Contains(stderrBuf.String(), "port is already allocated") {
			return fmt.Errorf("port %d is already in use by another process", port)
		}
		return fmt.Errorf("failed to start %s emulator: %w", emulator.Name, err)
	}

	fmt.Println()
	fmt.Printf("✅ %s emulator is running!\n\n", emulator.Name)
	fmt.Printf("  Container: %s\n", containerName)
	fmt.Printf("  Port:      %d\n", port)
	fmt.Println()

	printCloudEnvVars(emulator, hostPort)

	fmt.Printf("\n  Stop:   %s stop %s\n", runtime, containerName)
	fmt.Printf("  Remove: devx cloud rm %s\n", serviceName)
	return nil
}

// buildGCSArgs constructs container run arguments, with service-specific
// overrides for emulators that need custom entrypoints (e.g. fake-gcs-server).
func buildGCSArgs(serviceName, runtime, containerName string, port int, emulator cloud.Emulator) []string {
	runArgs := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:%d", port, emulator.InternalPort),
		"--label", "managed-by=devx",
		"--label", fmt.Sprintf("devx-cloud=%s", serviceName),
		"--restart", "unless-stopped",
	}

	for k, v := range emulator.Env {
		runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	runArgs = append(runArgs, emulator.Image)

	// Service-specific entrypoint overrides
	switch serviceName {
	case "gcs":
		// fake-gcs-server needs -scheme http -port and -backend memory flags
		runArgs = append(runArgs,
			"-scheme", "http",
			"-port", fmt.Sprintf("%d", emulator.InternalPort),
			"-backend", "memory",
			"-public-host", fmt.Sprintf("localhost:%d", port),
		)
	case "pubsub":
		runArgs = append(runArgs,
			"gcloud", "beta", "emulators", "pubsub", "start",
			fmt.Sprintf("--host-port=0.0.0.0:%d", emulator.InternalPort),
		)
	case "firestore":
		runArgs = append(runArgs,
			"gcloud", "beta", "emulators", "firestore", "start",
			fmt.Sprintf("--host-port=0.0.0.0:%d", emulator.InternalPort),
		)
	}

	return runArgs
}

// discoverCloudEmulatorEnvs finds all running devx-cloud-* containers and
// returns the combined set of SDK env vars for injection into devx shell.
func discoverCloudEmulatorEnvs(runtime string) map[string]string {
	out, err := exec.Command(runtime, "ps",
		"--filter", "label=managed-by=devx",
		"--filter", "label=devx-cloud",
		"--format", "{{.Names}}\t{{.Ports}}",
	).Output()
	if err != nil {
		return nil
	}

	result := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 1 || parts[0] == "" {
			continue
		}
		name := parts[0]
		serviceName := strings.TrimPrefix(name, "devx-cloud-")
		emulator, ok := cloud.Registry[serviceName]
		if !ok {
			continue
		}

		// Parse the host port from the port binding string "0.0.0.0:4443->4443/tcp"
		hostPort := fmt.Sprintf("localhost:%d", emulator.DefaultPort)
		if len(parts) == 2 && parts[1] != "" {
			for _, p := range strings.Split(parts[1], ", ") {
				if idx := strings.Index(p, "->"); idx > 0 {
					hostPort = "localhost:" + strings.TrimPrefix(p[:idx], "0.0.0.0:")
					break
				}
			}
		}

		for k, v := range emulator.EnvVarValues(hostPort) {
			result[k] = v
		}
	}
	return result
}

func printCloudEnvVars(emulator cloud.Emulator, hostPort string) {
	envVars := emulator.EnvVarValues(hostPort)
	if outputJSON {
		type result struct {
			Service string            `json:"service"`
			EnvVars map[string]string `json:"env_vars"`
		}
		b, _ := json.MarshalIndent(result{Service: emulator.Service, EnvVars: envVars}, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("  Add to your .env:")
	fmt.Println()
	for k, v := range envVars {
		fmt.Printf("    %s=%s\n", k, v)
	}
	fmt.Println()
	fmt.Println("  Or use 'devx shell' to have these injected automatically.")
}
