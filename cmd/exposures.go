package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/config"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/spf13/cobra"
)

var exposuresCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"exposures", "ls"},
	Short:   "List active port exposures",
	RunE: func(cmd *cobra.Command, args []string) error {
		devName := os.Getenv("USER")

		cfg := config.New(devName, "", "", "")
		if s, err := secrets.Load(envFile); err == nil {
			if s.DevHostname != "" {
				cfg.DevHostname = s.DevHostname
			}
		}

		if err := cloudflare.CheckLogin(); err != nil {
			return fmt.Errorf("cloudflared missing credentials: %w", err)
		}

		tunnels, err := cloudflare.ListExposedTunnels(devName)
		if err != nil {
			return fmt.Errorf("failed listing tunnels: %w", err)
		}

		if len(tunnels) == 0 {
			fmt.Println("No active exposures found. Run 'devx tunnel expose [port]' to create one.")
			return nil
		}

		fmt.Println("🌍 Active Cloudflare Exposures:")
		prefix := fmt.Sprintf("devx-expose-%s-", devName)
		for _, t := range tunnels {
			exposeID := strings.TrimPrefix(t.Name, prefix)
			fullDomain := exposure.GenerateDomain(exposeID, cfg.CFDomain)
			port := exposure.LookupPort(t.Name)
			if port == "" {
				port = "unknown"
			}
			fmt.Printf("  • %s\n", t.Name)
			fmt.Printf("    URL:      https://%s\n", fullDomain)
			fmt.Printf("    Port:     %s\n", port)
			fmt.Printf("    ID:       %s\n", t.ID)
			fmt.Printf("    Created:  %s\n\n", t.CreatedAt)
		}

		return nil
	},
}

func init() {
	tunnelCmd.AddCommand(exposuresCmd)
}
