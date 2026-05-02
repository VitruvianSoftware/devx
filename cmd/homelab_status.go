package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/cluster"
)

func newHomelabStatusCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current cluster state",
		Long: `Status displays the state of every configured node, including:

  - VM status (running, stopped, not created)
  - K3s role (server, agent, not installed)
  - Kubernetes node status (Ready, NotReady)
  - Architecture and pool labels`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			return cluster.Status(cmd.Context(), cfg)
		},
	}
}
