package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/cluster"
	"github.com/VitruvianSoftware/devx/internal/homelab/config"
)

func newHomelabApplyCmd(configFile *string) *cobra.Command {


	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply configuration changes iteratively with zero downtime",
		Long: `Applies changes made in homelab.yaml to a running cluster. 
It rolls out backend alterations like CPU and Memory changes node-by-node by:
1. Draining a node
2. Stopping the VM
3. Overwriting hardware configurations natively
4. Restarting the VM
5. Uncordoning the node`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			// Apply operations can take a while so we give it no strict overarching timeout
			// by default, or just use context.Background()
			ctx := context.Background()
			return cluster.Apply(ctx, cfg, DryRun)
		},
	}


	return cmd
}
