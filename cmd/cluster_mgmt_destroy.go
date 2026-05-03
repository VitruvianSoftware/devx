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

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/cluster"
)

func newClusterDestroyCmd(configFile *string) *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: "Tear down the entire cluster",
		Long: `Destroy uninstalls K3s from all nodes, stops and deletes all Lima VMs,
and removes the exported kubeconfig. Use -y/--non-interactive to skip confirmation.`,
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(c.Context(), timeout)
			return cluster.Destroy(ctx, cfg, NonInteractive, DryRun)
		},
	}


	cmd.Flags().DurationVar(&timeout, "timeout", 15*time.Minute, "maximum time for the entire operation")

	return cmd
}
