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

	"github.com/VitruvianSoftware/devx/internal/tui"
	"github.com/spf13/cobra"
)

var mailListRuntime string

var mailListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the devx-managed mail catcher",
	RunE:    runMailList,
}

func init() {
	mailListCmd.Flags().StringVar(&mailListRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	mailCmd.AddCommand(mailListCmd)
}

func runMailList(_ *cobra.Command, _ []string) error {
	runtime := mailListRuntime

	out, err := exec.Command(runtime, "ps", "-a",
		"--filter", "label=managed-by=devx",
		"--filter", "label=devx-mail",
		"--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}",
	).Output()
	if err != nil {
		return fmt.Errorf("listing mail containers: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		if outputJSON {
			fmt.Println("[]")
			return nil
		}
		fmt.Println(tui.StyleMuted.Render("  No mail catcher running. Use 'devx mail spawn' to start one."))
		return nil
	}

	type mailListJSON struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Ports  string `json:"ports"`
	}
	var results []mailListJSON

	if !outputJSON {
		fmt.Println(tui.StyleTitle.Render("devx — Mail Catcher") + "\n")
	}

	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		name, status, ports := parts[0], parts[1], ""
		if len(parts) == 3 {
			ports = parts[2]
		}

		if outputJSON {
			results = append(results, mailListJSON{Name: name, Status: status, Ports: ports})
			continue
		}

		statusStyle := tui.StyleDetailDone
		if !strings.Contains(strings.ToLower(status), "up") {
			statusStyle = tui.StyleDetailError
		}
		fmt.Printf("  %s  %s  %s  %s\n",
			tui.StyleLabel.Render("MailHog"),
			tui.StyleStepName.Render(name),
			statusStyle.Render(status),
			tui.StyleMuted.Render(ports),
		)
		// Show the env vars that are/would be injected
		envs := discoverMailEnvVars(runtime)
		for k, v := range envs {
			fmt.Printf("    %s  %s=%s\n",
				tui.StyleMuted.Render("env:"),
				tui.StyleLabel.Render(k),
				v,
			)
		}
	}

	if outputJSON {
		b, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Println()
	}
	return nil
}
