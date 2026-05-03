// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/cluster"
)

func newClusterInitCmd(configFile *string) *cobra.Command {
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
			_, _ = fmt.Fprintln(os.Stderr, "\n⚠️  Received interrupt signal, cancelling...")
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigCh)
	}()

	return ctx
}
