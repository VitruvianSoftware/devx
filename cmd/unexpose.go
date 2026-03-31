package cmd

import (
	"fmt"
	"os"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/spf13/cobra"
)

var unexposeCmd = &cobra.Command{
	Use:   "unexpose",
	Short: "Clean up all exposed localhost ports routed through Cloudflare.",
	RunE: func(cmd *cobra.Command, args []string) error {
		devName := os.Getenv("USER")

		if err := cloudflare.CheckLogin(); err != nil {
			return devxerr.New(devxerr.CodeNotLoggedIn, "Cloudflare credentials missing. Run 'cloudflared tunnel login'", err)
		}

		fmt.Println("🧹 Cleaning up exposed Cloudflare applications...")

		if DryRun {
			fmt.Println("DRY RUN: Would list and delete all active tunnels matching devx-expose-" + devName + "-*")
			return nil
		}
		
		count, err := cloudflare.CleanupExposedTunnels(devName)
		if err != nil {
			return fmt.Errorf("failed cleaning tunnels: %w", err)
		}

		fmt.Printf("✓ Successfully removed %d exposed tunnels.\n", count)
		_ = exposure.RemoveAll()
		return nil
	},
}

func init() {
	tunnelCmd.AddCommand(unexposeCmd)
}
