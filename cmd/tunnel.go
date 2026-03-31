package cmd

import "github.com/spf13/cobra"

var tunnelCmd = &cobra.Command{
	Use:   "tunnel",
	Short: "Manage Cloudflare tunnels and port exposure",
	Long: `Commands for exposing local ports to the internet via Cloudflare tunnels
and managing tunnel credentials.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
