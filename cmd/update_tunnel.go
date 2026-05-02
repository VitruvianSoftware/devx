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

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/secrets"
)

var updateTunnelCmd = &cobra.Command{
	Use:   "update",
	Short: "Rotate Cloudflare credentials without rebuilding the VM",
	Long:  "Gets a fresh tunnel token and restarts cloudflared inside the running VM.",
	RunE:  runUpdateTunnel,
}

func init() {
	tunnelCmd.AddCommand(updateTunnelCmd)
}

func runUpdateTunnel(_ *cobra.Command, _ []string) error {
	vm, err := getVMProvider()
	if err != nil {
		return err
	}

	devName := os.Getenv("USER")
	if devName == "" {
		devName = "developer"
	}
	cfg := config.New(devName, "", "", "")

	// Load current secrets
	s, err := secrets.Load(envFile)
	if err != nil {
		return fmt.Errorf("secrets: %w", err)
	}
	cfg.DevHostname = s.DevHostname

	if !vm.IsRunning(cfg.DevHostname) {
		return fmt.Errorf("VM %q is not running — use 'devx init' to provision it first", cfg.DevHostname)
	}

	fmt.Printf("Fetching tunnel token for %s...\n", cfg.TunnelName)
	token, err := cloudflare.GetToken(cfg.TunnelName)
	if err != nil {
		return fmt.Errorf("getting tunnel token: %w", err)
	}

	fmt.Println("Updating token inside VM...")
	updateCmd := fmt.Sprintf(
		`sudo sed -i 's|CF_TUNNEL_TOKEN=.*|CF_TUNNEL_TOKEN=%s|' /etc/dev-secrets.env`,
		token,
	)
	if _, err := vm.SSH(cfg.DevHostname, updateCmd); err != nil {
		return fmt.Errorf("updating token in VM: %w", err)
	}

	fmt.Println("Restarting cloudflared service...")
	if _, err := vm.SSH(cfg.DevHostname, "sudo systemctl restart cloudflared"); err != nil {
		return fmt.Errorf("restarting cloudflared: %w", err)
	}

	fmt.Printf("✓ Cloudflare tunnel credentials rotated for %s\n", cfg.CFDomain)
	return nil
}
