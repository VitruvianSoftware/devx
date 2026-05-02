package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/cluster"
)

func newHomelabRemoveCmd(configFile *string) *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "remove <node-host>",
		Short: "Drain and remove a node from the cluster",
		Long: `Remove drains the specified node, uninstalls K3s, and optionally
destroys the Lima VM on the target host.`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(c.Context(), timeout)
			return cluster.Remove(ctx, cfg, args[0], DryRun)
		},
	}


	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "maximum time for the entire operation")

	return cmd
}
