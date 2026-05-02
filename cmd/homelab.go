package cmd

import (
	"github.com/spf13/cobra"
)

var homelabConfigFile string

var homelabCmd = &cobra.Command{
	Use:   "homelab",
	Short: "Declaratively provision and manage multi-node K8s homelab clusters on macOS",
	Long: `homelab is a CLI tool for provisioning and managing Kubernetes clusters
across multiple macOS machines using Lima VZ and K3s.

Define your cluster topology in a YAML config file, then use homelab to
bootstrap, scale, diagnose, and tear down your cluster.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	homelabCmd.PersistentFlags().StringVarP(&homelabConfigFile, "config", "c", "homelab.yaml", "path to cluster config file")

	homelabCmd.AddCommand(
		newHomelabInitCmd(&homelabConfigFile),
		newHomelabApplyCmd(&homelabConfigFile),
		newHomelabJoinCmd(&homelabConfigFile),
		newHomelabRemoveCmd(&homelabConfigFile),
		newHomelabDestroyCmd(&homelabConfigFile),
		newHomelabUpgradeCmd(&homelabConfigFile),
		newHomelabBackupCmd(&homelabConfigFile),
		newHomelabRestoreCmd(&homelabConfigFile),
		newHomelabDoctorCmd(&homelabConfigFile),
		newHomelabStatusCmd(&homelabConfigFile),
	)

	// Will add this to rootCmd in root.go
}
