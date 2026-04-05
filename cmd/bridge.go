package cmd

import (
	"github.com/spf13/cobra"
)

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
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
