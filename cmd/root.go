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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/ai"
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
var DetailedOutput bool

var rootCmd = &cobra.Command{
	Use:   "devx",
	Short: "Supercharged local dev environment",
	Long: `devx is the unified orchestration layer for your modern developer lifecycle.
It replaces a fragmented toolchain with a single, declarative CLI.

Configuration Domains:
  • devx.yaml:    Project-level configuration for local application development.
                  Controls databases, tunnels, CI, and service orchestration.
                  (Discovered recursively upward from the current directory)
  • cluster.yaml: Infrastructure-level config for multi-node Kubernetes clusters.
                  Exclusively controls the 'devx cluster' command suite.
                  (Discovered recursively upward from the current directory)

Feature Ecosystem:
  • Local Infrastructure: Podman VMs, ephemeral databases, AI synthetic data, GCP emulators, and The Nuke button.
  • Kubernetes & Hybrid: Zero-Config k3s, Multi-Node Clusters, and remote Hybrid Bridge.
  • Networking & Edge: Instant Cloudflare Tunnels, Email Catchers, and Webhook TUIs.
  • Orchestration & State: Multi-repo management, Vault secrets syncing, and file syncing.
  • Testing & Telemetry: Ephemeral E2E, distributed tracing, API mocking, and Doctor checks.
  • Pipelines & CI/CD: Local GitHub Actions emulation, pipeline stages, and predictive building.

Run 'devx vm init' to bootstrap your machine, or 'devx up' to start services.`,
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
				_, _ = fmt.Fprintf(os.Stderr, "\n╭─────────────────────────────────────────────────╮\n")
				_, _ = fmt.Fprintf(os.Stderr, "│  ✦ devx %s is available (you have %s)  │\n", result.Latest, result.Current)
				_, _ = fmt.Fprintf(os.Stderr, "│    Run: devx upgrade                            │\n")
				_, _ = fmt.Fprintf(os.Stderr, "╰─────────────────────────────────────────────────╯\n")
			}
		default:
			// Check hasn't finished yet — skip silently
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var dex *devxerr.DevxError
		exitCode := 1
		if errors.As(err, &dex) {
			exitCode = dex.ExitCode
		}

		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		// Idea 59: Attempt intelligent failure diagnosis before exiting.
		// Suppressed in --json mode so AI agents parsing structured output
		// don't get free-form diagnosis text injected.
		if !outputJSON {
			cmdName := ""
			if len(os.Args) > 1 {
				cmdName = strings.Join(os.Args[1:], " ")
			}
			if diagnosis := ai.DiagnoseFailure(cmdName, exitCode, err.Error(), ""); diagnosis != "" {
				_, _ = fmt.Fprintf(os.Stderr, "\n%s\n", diagnosis)
			}
		}

		os.Exit(exitCode)
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
	rootCmd.PersistentFlags().BoolVar(&DetailedOutput, "detailed", false,
		"Enable detailed Go test output (shows individual passing tests instead of just package summaries)")

	rootCmd.AddGroup(
		&cobra.Group{ID: "infra", Title: "Local Infrastructure:"},
		&cobra.Group{ID: "k8s", Title: "Kubernetes & Hybrid:"},
		&cobra.Group{ID: "network", Title: "Networking & Edge:"},
		&cobra.Group{ID: "orchestration", Title: "Orchestration & State:"},
		&cobra.Group{ID: "telemetry", Title: "Testing & Telemetry:"},
		&cobra.Group{ID: "ci", Title: "Pipelines & CI/CD:"},
	)
	tunnelCmd.AddGroup(&cobra.Group{ID: "orchestration", Title: "Orchestration & State:"})

	rootCmd.AddCommand(vmCmd)
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(bridgeCmd)
	rootCmd.AddCommand(clusterMgmtCmd)
}
