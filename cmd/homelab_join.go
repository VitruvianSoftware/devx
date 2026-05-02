package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/cluster"
)

func newHomelabJoinCmd(configFile *string) *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Add new node(s) to an existing cluster",
		Long: `Join adds nodes that are defined in the config but not yet part of the cluster.
It detects which nodes are missing and provisions only those.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(cmd.Context(), timeout)
			return cluster.Join(ctx, cfg, DryRun)
		},
	}


	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "maximum time for the entire operation")

	return cmd
}
