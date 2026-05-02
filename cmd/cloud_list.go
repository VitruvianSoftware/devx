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
	"os/exec"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/cloud"
	"github.com/VitruvianSoftware/devx/internal/tui"
	"github.com/spf13/cobra"
)

var cloudListRuntime string

var cloudListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all devx-managed cloud emulators",
	Aliases: []string{"ls"},
	RunE:    runCloudList,
}

func init() {
	cloudListCmd.Flags().StringVar(&cloudListRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	cloudCmd.AddCommand(cloudListCmd)
}

func runCloudList(_ *cobra.Command, _ []string) error {
	runtime := cloudListRuntime

	cmd := exec.Command(runtime, "ps", "-a",
		"--filter", "label=managed-by=devx",
		"--filter", "label=devx-cloud",
		"--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}\t{{.Labels}}")

	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("listing containers: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		if outputJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Println(tui.StyleMuted.Render("  No cloud emulators running. Use 'devx cloud spawn <service>' to start one."))
		fmt.Printf("  Supported: %s\n", strings.Join(cloud.SupportedServices(), ", "))
		return nil
	}

	if !outputJSON {
		fmt.Println(tui.StyleTitle.Render("devx — Cloud Emulators") + "\n")
	}

	type cloudJSON struct {
		Name    string            `json:"name"`
		Status  string            `json:"status"`
		Ports   string            `json:"ports"`
		Service string            `json:"service"`
		EnvVars map[string]string `json:"env_vars"`
	}
	var cloudList []cloudJSON

	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 3 {
			continue
		}
		name := parts[0]
		status := parts[1]
		ports := parts[2]

		statusStyle := tui.StyleDetailDone
		if !strings.Contains(strings.ToLower(status), "up") {
			statusStyle = tui.StyleDetailError
		}

		// Extract the service name from the label (devx-cloud=gcs)
		serviceName := strings.TrimPrefix(name, "devx-cloud-")
		emulator, ok := cloud.Registry[serviceName]
		displayName := serviceName
		if ok {
			displayName = emulator.Name
		}

		// Parse the local port from the ports string to build env vars
		hostPort := fmt.Sprintf("localhost:%d", emulator.DefaultPort)
		if ports != "" {
			// ports string looks like "0.0.0.0:4443->4443/tcp"
			for _, p := range strings.Split(ports, ", ") {
				if idx := strings.Index(p, "->"); idx > 0 {
					hostPort = "localhost:" + strings.TrimPrefix(p[:idx], "0.0.0.0:")
					break
				}
			}
		}

		envVars := map[string]string{}
		if ok {
			envVars = emulator.EnvVarValues(hostPort)
		}

		if outputJSON {
			cloudList = append(cloudList, cloudJSON{
				Name:    name,
				Status:  status,
				Ports:   ports,
				Service: displayName,
				EnvVars: envVars,
			})
			continue
		}

		fmt.Printf("  %s  %s  %s  %s\n",
			tui.StyleLabel.Render(displayName),
			tui.StyleStepName.Render(name),
			statusStyle.Render(status),
			tui.StyleMuted.Render(ports),
		)
		for k, v := range envVars {
			fmt.Printf("    %s  %s=%s\n",
				tui.StyleMuted.Render("env:"),
				tui.StyleLabel.Render(k),
				v,
			)
		}
	}

	if outputJSON {
		enc, _ := json.MarshalIndent(cloudList, "", "  ")
		fmt.Println(string(enc))
	} else {
		fmt.Println()
	}

	return nil
}
