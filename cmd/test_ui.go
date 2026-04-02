package cmd

import (
	"fmt"
	"os"
	"strings"

	ephemeraltest "github.com/VitruvianSoftware/devx/internal/testing"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	testUISetup   string
	testUICommand string
	testUIRuntime string
)

var testUICmd = &cobra.Command{
	Use:   "ui [-- command]",
	Short: "Run UI/E2E tests against an ephemeral, isolated topology",
	Long: `Run Playwright, Cypress, or any browser-based E2E test against a
pristine, ephemeral database topology that is automatically provisioned and
destroyed around the test run.

The commands, setup steps, and databases are read from devx.yaml (test.ui.*),
but any value can be overridden via CLI flags for one-off use.

Examples:

  # Use configuration from devx.yaml
  devx test ui

  # Override the test command inline (CLI takes precedence over devx.yaml)
  devx test ui --command "npx playwright test"

  # Include a migration step before running tests
  devx test ui --setup "npm run db:migrate" --command "npx playwright test"

  # Pass extra arguments to the test runner
  devx test ui -- npx playwright test --reporter=html`,
	RunE: runTestUI,
}

func init() {
	testUICmd.Flags().StringVar(&testUISetup, "setup", "",
		"Shell command to run before tests (e.g., 'npm run db:migrate'). Overrides devx.yaml test.ui.setup.")
	testUICmd.Flags().StringVar(&testUICommand, "command", "",
		"Test command to execute. Overrides devx.yaml test.ui.command.")
	testUICmd.Flags().StringVar(&testUIRuntime, "runtime", "podman",
		"Container runtime for ephemeral databases (podman or docker).")
	testCmd.AddCommand(testUICmd)
}

func runTestUI(cmd *cobra.Command, args []string) error {
	// If extra args were passed after --, treat them as the command
	if len(args) > 0 {
		testUICommand = strings.Join(args, " ")
	}

	// Load devx.yaml for database topology and YAML-level test config
	yamlPath := "devx.yaml"
	var cfgYaml DevxConfig

	if b, err := os.ReadFile(yamlPath); err == nil {
		if err := yaml.Unmarshal(b, &cfgYaml); err != nil {
			return fmt.Errorf("failed to parse devx.yaml: %w", err)
		}
	}

	// CLI flags take precedence over YAML values (CLI + YAML parity)
	resolvedSetup := testUISetup
	if resolvedSetup == "" {
		resolvedSetup = cfgYaml.Test.UI.Setup
	}
	resolvedCommand := testUICommand
	if resolvedCommand == "" {
		resolvedCommand = cfgYaml.Test.UI.Command
	}

	// Collect engine names from devx.yaml databases block
	var dbEngines []string
	for _, db := range cfgYaml.Databases {
		if db.Engine != "" {
			dbEngines = append(dbEngines, db.Engine)
		}
	}

	runtime := testUIRuntime
	if runtime != "podman" && runtime != "docker" {
		return fmt.Errorf("unsupported runtime %q — use 'podman' or 'docker'", runtime)
	}

	return ephemeraltest.RunFlow(dbEngines, runtime, resolvedSetup, resolvedCommand)
}
