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

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/multinode/cluster"
	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

func newClusterApplyCmd(configFile *string) *cobra.Command {


	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply configuration changes iteratively with zero downtime",
		Long: `Applies changes made in cluster.yaml to a running cluster. 
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
