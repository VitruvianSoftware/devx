package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/doctor"
)

func newHomelabDoctorCmd(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run pre-flight checks and health diagnostics",
		Long: `Doctor verifies that all prerequisites are met on each configured host
and, if a cluster is running, checks its health. Checks include:

  - SSH connectivity to all hosts
  - Homebrew installation
  - Lima installation and version
  - socket_vmnet installation and service status
  - VM provisioning state
  - Network bridging and cross-VM connectivity
  - K3s health, node readiness, and etcd quorum`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			return doctor.Run(cmd.Context(), cfg)
		},
	}
}
