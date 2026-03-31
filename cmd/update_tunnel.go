package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/podman"
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

	if !podman.IsRunning(cfg.DevHostname) {
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
	if _, err := podman.SSH(cfg.DevHostname, updateCmd); err != nil {
		return fmt.Errorf("updating token in VM: %w", err)
	}

	fmt.Println("Restarting cloudflared service...")
	if _, err := podman.SSH(cfg.DevHostname, "sudo systemctl restart cloudflared"); err != nil {
		return fmt.Errorf("restarting cloudflared: %w", err)
	}

	fmt.Printf("✓ Cloudflare tunnel credentials rotated for %s\n", cfg.CFDomain)
	return nil
}
