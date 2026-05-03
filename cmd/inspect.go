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
	"math/rand"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/authproxy"
	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/inspector"
	"github.com/VitruvianSoftware/devx/internal/secrets"
)

var (
	inspectExpose    bool
	inspectName      string
	inspectDomain    string
	inspectBasicAuth string
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [port]",
	Short: "Inspect and replay HTTP traffic with a live TUI",
	Long: `Starts a reverse proxy that captures all HTTP request/response pairs and
displays them in a live terminal interface. Optionally expose the proxy
via a Cloudflare tunnel with --expose.

This is a free, open-source replacement for ngrok's paid web inspector.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port := args[0]

		// Start the inspector proxy
		captureCh := make(chan inspector.CapturedExchange, 256)
		proxy, err := inspector.NewProxy(port, func(ex inspector.CapturedExchange) {
			// Non-blocking send; drop captures if TUI can't keep up
			select {
			case captureCh <- ex:
			default:
			}
		})
		if err != nil {
			return fmt.Errorf("failed to create inspector proxy: %w", err)
		}

		proxyPort, err := proxy.Start()
		if err != nil {
			return fmt.Errorf("failed to start inspector proxy: %w", err)
		}
		defer func() { _ = proxy.Close() }() 

		tunnelURL := ""

		// Optionally expose via Cloudflare tunnel
		var cfProcess *os.Process
		var tunnelName, tunnelID string

		if inspectExpose || inspectName != "" {
			tunnelURL, tunnelName, tunnelID, cfProcess, err = setupTunnel(proxyPort)
			if err != nil {
				return err
			}
			defer cleanupTunnel(cfProcess, tunnelName, tunnelID)
		}

		// Launch the TUI
		model := inspector.NewTUIModel(proxy, captureCh, tunnelURL, port, proxyPort)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

// setupTunnel creates a Cloudflare tunnel pointing at the proxy port.
func setupTunnel(proxyPort int) (tunnelURL string, tunnelName string, tunnelID string, proc *os.Process, err error) {
	devName := os.Getenv("USER")

	cfg := config.New(devName, "", "", "")
	if s, loadErr := secrets.Load(envFile); loadErr == nil {
		if s.DevHostname != "" {
			cfg.DevHostname = s.DevHostname
		}
	}

	baseDomain := cfg.CFDomain
	if inspectDomain != "" {
		baseDomain = inspectDomain
	} else if baseDomain == "" {
		return "", "", "", nil, fmt.Errorf("CFDomain is not configured. Run `devx vm init` or `devx config secrets` first")
	}

	if err = ensureCloudflareLogin(); err != nil {
		return "", "", "", nil, err
	}

	exposeID := inspectName
	if exposeID == "" {
		exposeID = fmt.Sprintf("inspect-%x", int(rand.Int31()&0xfffff))
	}

	fullDomain := exposure.GenerateDomain(exposeID, baseDomain)
	tunnelName = fmt.Sprintf("devx-expose-%s-%s", devName, exposeID)

	tunnel, err := cloudflare.EnsureTunnel(tunnelName)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed creating tunnel: %w", err)
	}

	if err = cloudflare.RouteDNS(tunnelName, fullDomain); err != nil {
		return "", "", "", nil, fmt.Errorf("failed routing DNS: %w", err)
	}

	// Persist exposure state
	_ = exposure.Save(exposure.Entry{
		TunnelName: tunnelName,
		TunnelID:   tunnel.ID,
		Port:       fmt.Sprintf("%d", proxyPort),
		Domain:     fullDomain,
	})

	targetPort := fmt.Sprintf("%d", proxyPort)
	if inspectBasicAuth != "" {
		newPort, cleanupAuth, err := authproxy.Start(targetPort, inspectBasicAuth)
		if err != nil {
			return "", "", "", nil, fmt.Errorf("failed creating basic auth proxy: %w", err)
		}
		// Since we defer cleanupTunnel later, we can't cleanly defer cleanupAuth returning it,
		// but since authproxy will die when the devx CLI dies, it's fine.
		_ = cleanupAuth
		targetPort = fmt.Sprintf("%d", newPort)
	}

	configFile, err := cloudflare.WriteIngressConfig(tunnel.ID, fullDomain, targetPort)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to create ingress config: %w", err)
	}

	// Run cloudflared in background
	cfCmd := exec.Command("cloudflared", "tunnel", "--config", configFile, "run")
	cfCmd.Stdout = nil
	cfCmd.Stderr = nil
	if err = cfCmd.Start(); err != nil {
		return "", "", "", nil, fmt.Errorf("failed starting cloudflared: %w", err)
	}

	tunnelURL = "https://" + fullDomain
	return tunnelURL, tunnelName, tunnel.ID, cfCmd.Process, nil
}

// cleanupTunnel kills cloudflared and removes the tunnel.
func cleanupTunnel(proc *os.Process, tunnelName, tunnelID string) {
	if proc != nil {
		_ = proc.Kill()
		_, _ = proc.Wait()
	}
	if tunnelName != "" {
		_, _ = exec.Command("cloudflared", "tunnel", "delete", "-f", tunnelName).CombinedOutput()
		_ = exposure.RemoveByName(tunnelName)
	}
	if tunnelID != "" {
		home, err := os.UserHomeDir()
		if err == nil {
			_ = os.Remove(fmt.Sprintf("%s/.cloudflared/%s.json", home, tunnelID))
			_ = os.Remove(fmt.Sprintf("%s/.cloudflared/%s-config.yml", home, tunnelID))
		}
	}
}

func init() {
	inspectCmd.Flags().BoolVarP(&inspectExpose, "expose", "e", false, "Expose the inspected port via Cloudflare tunnel")
	inspectCmd.Flags().StringVarP(&inspectName, "name", "n", "", "Static sub-domain name to use if exposing")
	inspectCmd.Flags().StringVar(&inspectDomain, "domain", "", "Custom Cloudflare domain (BYOD) (e.g. 'mycompany.dev')")
	inspectCmd.Flags().StringVar(&inspectBasicAuth, "basic-auth", "", "Protect the exposed tunnel with basic auth (e.g. 'user:pass')")
	tunnelCmd.AddCommand(inspectCmd)
}
