package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/spf13/cobra"
)

var envFile string
var outputJSON bool
var NonInteractive bool
var DryRun bool

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
	SilenceErrors: true, // we handle it in Execute()
	SilenceUsage:  true, // don't dump help text on errors
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var dex *devxerr.DevxError
		if errors.As(err, &dex) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(dex.ExitCode)
		}
		
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(func() {
		secrets.NonInteractive = NonInteractive
	})

	rootCmd.PersistentFlags().StringVar(&envFile, "env-file", ".env",
		"Path to secrets file (default: .env in current directory)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false,
		"Output results in machine-readable JSON format for AI agents")
	rootCmd.PersistentFlags().BoolVarP(&NonInteractive, "non-interactive", "y", false,
		"Bypass interactive prompts and auto-confirm destructive actions")
	rootCmd.PersistentFlags().BoolVar(&DryRun, "dry-run", false,
		"Print what destructive actions would do without executing them")

	rootCmd.AddCommand(vmCmd)
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(execCmd)
}
