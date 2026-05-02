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
	"os"
	"os/exec"

	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/spf13/cobra"
)

var tailscaleCmd = &cobra.Command{
	Use:                "tailscale [args...]",
	Short:              "Pass-through command for Tailscale interacting explicitly with the VM daemon.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		prov, err := getFullProvider()
		if err != nil {
			return err
		}

		devName := os.Getenv("USER")
		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil {
			cfg.DevHostname = s.DevHostname
		}

		var pCmd *exec.Cmd

		switch prov.VM.Name() {
		case "lima":
			rtArgs := append([]string{"shell", cfg.DevHostname, "sudo", "nerdctl", "exec", "-it", "tailscaled", "tailscale"}, args...)
			pCmd = exec.Command("limactl", rtArgs...)
		case "colima":
			rtArgs := append([]string{"ssh", "--", "sudo", "nerdctl", "exec", "-it", "tailscaled", "tailscale"}, args...)
			pCmd = exec.Command("colima", rtArgs...)
		case "podman":
			rtArgs := append([]string{"machine", "ssh", cfg.DevHostname, "sudo", "podman", "exec", "-it", "tailscaled", "tailscale"}, args...)
			pCmd = exec.Command("podman", rtArgs...)
		default: // docker, orbstack (fallback to nsenter)
			rtArgs := append([]string{"run", "--rm", "-it", "--privileged", "--pid=host", "alpine:latest", "nsenter", "-t", "1", "-m", "-u", "-n", "-i", "docker", "exec", "-it", "tailscaled", "tailscale"}, args...)
			pCmd = exec.Command("docker", rtArgs...)
		}

		pCmd.Stdin = os.Stdin
		pCmd.Stdout = os.Stdout
		pCmd.Stderr = os.Stderr
		return pCmd.Run()
	},
}

func init() {
	execCmd.AddCommand(tailscaleCmd)
}
