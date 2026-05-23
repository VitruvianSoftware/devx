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
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/multinode/cluster"
	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

func newClusterReconcileCmd(configFile *string) *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Converge existing nodes to the current node baseline (e.g. required packages)",
		Long: `Reconcile brings already-provisioned nodes up to devx's current node baseline
without rebuilding them. Today that means ensuring the standard node package
set — including socat, which kubectl port-forward and devx bridge require to
carry traffic on Docker-runtime k3s nodes — is installed inside every node's
Lima VM.

It does not touch K3s or VM lifecycle, so a cluster created before a new
node-level requirement landed can adopt it without a destroy/recreate. It is
safe to run repeatedly; nodes that already satisfy the baseline are fast
no-ops. Use --dry-run to preview the per-node command without changing
anything.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(cmd.Context(), timeout)
			return cluster.Reconcile(ctx, cfg, DryRun)
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "maximum time for the entire operation")

	return cmd
}
