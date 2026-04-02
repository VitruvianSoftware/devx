package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/secrets"
	"github.com/VitruvianSoftware/devx/internal/updater"
	"github.com/spf13/cobra"
)

// updateResult receives the background version check result.
var updateResult = make(chan *updater.CheckResult, 1)

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
	// After every command, print an update notice if one is available.
	// Suppressed in --json mode so AI agents don't get noise.
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if outputJSON {
			return
		}
		select {
		case result := <-updateResult:
			if result != nil && result.UpdateAvailable {
				fmt.Fprintf(os.Stderr, "\n╭─────────────────────────────────────────────────╮\n")
				fmt.Fprintf(os.Stderr, "│  ✦ devx %s is available (you have %s)  │\n", result.Latest, result.Current)
				fmt.Fprintf(os.Stderr, "│    Run: devx upgrade                            │\n")
				fmt.Fprintf(os.Stderr, "╰─────────────────────────────────────────────────╯\n")
			}
		default:
			// Check hasn't finished yet — skip silently
		}
	},
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
		// Fire the update check in the background immediately.
		go func() {
			result, _ := updater.Check(version)
			select {
			case updateResult <- result:
			default:
			}
		}()
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
