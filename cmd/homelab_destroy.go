package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/cluster"
)

func newHomelabDestroyCmd(configFile *string) *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Tear down the entire cluster",
		Long: `Destroy uninstalls K3s from all nodes, stops and deletes all Lima VMs,
and removes the exported kubeconfig. Use -y/--non-interactive to skip confirmation.`,
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(c.Context(), timeout)
			return cluster.Destroy(ctx, cfg, NonInteractive, DryRun)
		},
	}


	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Minute, "maximum time for the entire operation")

	return cmd
}
