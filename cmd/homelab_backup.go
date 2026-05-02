package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/cluster"
)

func newHomelabBackupCmd(configFile *string) *cobra.Command {
	var (
		outputDir string
		timeout   time.Duration
	)

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create an etcd snapshot backup",
		Long: `Backup triggers a K3s etcd snapshot on the init control plane node
and downloads it to the local machine.`,
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(c.Context(), timeout)
			return cluster.Backup(ctx, cfg, outputDir)
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", ".", "directory to save the snapshot")
	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "maximum time for the operation")

	return cmd
}

func newHomelabRestoreCmd(configFile *string) *cobra.Command {
	var (
		snapshotPath string
		timeout      time.Duration
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore from an etcd snapshot",
		Long: `Restore stops K3s on the init node, restores etcd from the given
snapshot file, and restarts K3s.`,
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(c.Context(), timeout)
			return cluster.Restore(ctx, cfg, snapshotPath)
		},
	}

	cmd.Flags().StringVarP(&snapshotPath, "snapshot", "s", "", "path to the snapshot file")
	cobra.CheckErr(cmd.MarkFlagRequired("snapshot"))
	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Minute, "maximum time for the operation")

	return cmd
}
