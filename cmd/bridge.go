package cmd

import (
	"github.com/spf13/cobra"
)

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Connect your local environment to a remote Kubernetes cluster",
	Long: `Hybrid edge-to-local routing (Idea 46) — connect your local dev
environment to remote Kubernetes services via kubectl port-forward.

This enables you to run a local service that calls remote staging APIs
without modifying your application code. Bridge injects BRIDGE_*_URL
environment variables into devx shell automatically.

Commands:
  connect      Establish outbound bridge to remote cluster services
  status       Show active bridge sessions
  disconnect   Tear down all active bridges

Examples:
  # Connect using devx.yaml bridge config
  devx bridge connect

  # Connect with ad-hoc targets (no devx.yaml needed)
  devx bridge connect --context staging --target payments-api:8080 --target redis:6379

  # Check bridge status
  devx bridge status

  # Tear down everything
  devx bridge disconnect`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	bridgeCmd.AddCommand(bridgeConnectCmd)
	bridgeCmd.AddCommand(bridgeStatusCmd)
	bridgeCmd.AddCommand(bridgeDisconnectCmd)
}
