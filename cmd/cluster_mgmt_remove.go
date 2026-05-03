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

func newClusterRemoveCmd(configFile *string) *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "remove <node-host>",
		Short: "Drain and remove a node from the cluster",
		Long: `Remove drains the specified node, uninstalls K3s, and optionally
destroys the Lima VM on the target host.`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load(*configFile)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			ctx := contextWithSignal(c.Context(), timeout)
			return cluster.Remove(ctx, cfg, args[0], DryRun)
		},
	}


	cmd.Flags().DurationVar(&timeout, "timeout", 10*time.Minute, "maximum time for the entire operation")

	return cmd
}
