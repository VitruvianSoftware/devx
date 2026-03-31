package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/inspector"
	"github.com/VitruvianSoftware/devx/internal/secrets"
)

var (
	inspectExpose bool
	inspectName   string
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
		defer proxy.Close()

		tunnelURL := ""

		// Optionally expose via Cloudflare tunnel
		var cfProcess *os.Process
		var tunnelName string

		if inspectExpose || inspectName != "" {
			tunnelURL, tunnelName, cfProcess, err = setupTunnel(proxyPort)
			if err != nil {
				return err
			}
			defer cleanupTunnel(cfProcess, tunnelName)
		}

		// Launch the TUI
		model := inspector.NewTUIModel(proxy, captureCh, tunnelURL, port, proxyPort)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

// setupTunnel creates a Cloudflare tunnel pointing at the proxy port.
func setupTunnel(proxyPort int) (tunnelURL string, tunnelName string, proc *os.Process, err error) {
	devName := os.Getenv("USER")

	cfg := config.New(devName, "", "", "")
	if s, loadErr := secrets.Load(envFile); loadErr == nil {
		if s.DevHostname != "" {
			cfg.DevHostname = s.DevHostname
		}
	}

	if cfg.CFDomain == "" {
		return "", "", nil, fmt.Errorf("CFDomain is not configured. Run `devx vm init` or `devx config secrets` first")
	}

	if err = cloudflare.CheckLogin(); err != nil {
		return "", "", nil, fmt.Errorf("cloudflared missing credentials: %w", err)
	}

	exposeID := inspectName
	if exposeID == "" {
		exposeID = fmt.Sprintf("inspect-%x", int(rand.Int31()&0xfffff))
	}

	fullDomain := fmt.Sprintf("%s.%s", exposeID, cfg.CFDomain)
	tunnelName = fmt.Sprintf("devx-expose-%s-%s", devName, exposeID)

	tunnel, err := cloudflare.EnsureTunnel(tunnelName)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed creating tunnel: %w", err)
	}

	if err = cloudflare.RouteDNS(tunnelName, fullDomain); err != nil {
		return "", "", nil, fmt.Errorf("failed routing DNS: %w", err)
	}

	// Persist exposure state
	_ = exposure.Save(exposure.Entry{
		TunnelName: tunnelName,
		TunnelID:   tunnel.ID,
		Port:       fmt.Sprintf("%d", proxyPort),
		Domain:     fullDomain,
	})

	home, _ := os.UserHomeDir()
	credFile := fmt.Sprintf("%s/.cloudflared/%s.json", home, tunnel.ID)

	// Run cloudflared in background
	cfCmd := exec.Command("cloudflared", "tunnel", "run",
		"--credentials-file", credFile,
		"--url", fmt.Sprintf("http://localhost:%d", proxyPort),
		tunnel.ID,
	)
	cfCmd.Stdout = nil
	cfCmd.Stderr = nil
	if err = cfCmd.Start(); err != nil {
		return "", "", nil, fmt.Errorf("failed starting cloudflared: %w", err)
	}

	tunnelURL = "https://" + fullDomain
	return tunnelURL, tunnelName, cfCmd.Process, nil
}

// cleanupTunnel kills cloudflared and removes the tunnel.
func cleanupTunnel(proc *os.Process, tunnelName string) {
	if proc != nil {
		_ = proc.Kill()
		_, _ = proc.Wait()
	}
	if tunnelName != "" {
		_, _ = exec.Command("cloudflared", "tunnel", "delete", "-f", tunnelName).CombinedOutput()
		_ = exposure.RemoveByName(tunnelName)
	}
}

func init() {
	inspectCmd.Flags().BoolVar(&inspectExpose, "expose", false, "Expose the inspector via a Cloudflare tunnel")
	inspectCmd.Flags().StringVarP(&inspectName, "name", "n", "", "Static sub-domain name (implies --expose)")
	tunnelCmd.AddCommand(inspectCmd)
}
