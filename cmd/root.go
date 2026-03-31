package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var envFile string
var outputJSON bool

var rootCmd = &cobra.Command{
	Use:   "devx",
	Short: "Supercharged local dev environment (Podman + Cloudflare + Tailscale)",
	Long: `devx provisions a Fedora CoreOS Podman VM pre-configured with:
  • Cloudflare Tunnel — instant public HTTPS endpoint on *.ipv1337.dev
  • Tailscale — zero-trust access to the corporate Tailnet

Run 'devx vm init' to set up your environment for the first time.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Execute is the main entrypoint called from main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&envFile, "env-file", ".env",
		"Path to secrets file (default: .env in current directory)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false,
		"Output results in machine-readable JSON format for AI agents")

	rootCmd.AddCommand(vmCmd)
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(execCmd)
}
