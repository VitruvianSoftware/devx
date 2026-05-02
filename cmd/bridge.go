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
	"github.com/spf13/cobra"
)

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	GroupID: "network",
	Short: "Connect your local environment to a remote Kubernetes cluster",
	Long: `Hybrid edge-to-local routing (Idea 46) — connect your local dev
environment to remote Kubernetes services via kubectl port-forward,
and intercept inbound cluster traffic for live local debugging.

Commands:
  connect      Establish outbound bridge to remote cluster services
  intercept    Intercept inbound traffic from a remote service (Idea 46.2)
  status       Show active bridge and intercept sessions
  disconnect   Tear down all active bridges and intercepts
  rbac         Print minimum RBAC manifest for intercept

Examples:
  # Outbound: connect to remote services (Idea 46.1)
  devx bridge connect

  # Inbound: intercept live traffic (Idea 46.2)
  devx bridge intercept payments-api --steal

  # Check status of all bridges and intercepts
  devx bridge status

  # Tear down everything
  devx bridge disconnect`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	bridgeCmd.AddCommand(bridgeConnectCmd)
	bridgeCmd.AddCommand(bridgeInterceptCmd)
	bridgeCmd.AddCommand(bridgeStatusCmd)
	bridgeCmd.AddCommand(bridgeDisconnectCmd)
	bridgeCmd.AddCommand(bridgeRBACCmd)
}
