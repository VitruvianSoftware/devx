package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/homelab/config"
	"github.com/VitruvianSoftware/devx/internal/homelab/cluster"
)

func newHomelabInitCmd(configFile *string) *cobra.Command {
	var (
		autoInstall bool
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Bootstrap a new cluster from the config file",
		Long: `Initialize a new Kubernetes cluster by provisioning Lima VMs on each
configured host and bootstrapping K3s in HA mode.

This command is idempotent: it will skip any steps that have already been completed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(cmd.Context(), timeout)
			return cluster.Init(ctx, cfg, cluster.InitOptions{
				DryRun:      DryRun,
				AutoInstall: autoInstall,
			})
		},
	}


	cmd.Flags().BoolVar(&autoInstall, "auto-install", false, "automatically install missing prerequisites")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "maximum time for the entire operation")

	return cmd
}

// contextWithSignal creates a context that cancels on SIGINT/SIGTERM or after the given timeout.
func contextWithSignal(parent context.Context, timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(parent, timeout)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigCh:
			fmt.Fprintln(os.Stderr, "\n⚠️  Received interrupt signal, cancelling...")
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigCh)
	}()

	return ctx
}
