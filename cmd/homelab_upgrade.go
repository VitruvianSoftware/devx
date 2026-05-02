package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/cluster"
)

func newHomelabUpgradeCmd(configFile *string) *cobra.Command {
	var (
		dryRun  bool
		timeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Rolling upgrade K3s to the version specified in config",
		Long: `Upgrade performs a rolling upgrade of K3s across all cluster nodes.
Agent nodes are upgraded first, then server nodes one at a time to
maintain etcd quorum. Each node is drained before upgrade and
uncordoned after, with a health check between each node.`,
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(c.Context(), timeout)
			return cluster.Upgrade(ctx, cfg, dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would happen without making changes")
	cmd.Flags().DurationVar(&timeout, "timeout", 45*time.Minute, "maximum time for the entire operation")

	return cmd
}
