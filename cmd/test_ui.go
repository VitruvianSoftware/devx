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
	"fmt"
	"strings"

	ephemeraltest "github.com/VitruvianSoftware/devx/internal/testing"
	"github.com/spf13/cobra"
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
	// Idea 44: Use resolveConfig so include blocks are processed
	var cfgYaml DevxConfig
	if yamlPath, err := findDevxConfig(); err == nil {
		if cfg, err := resolveConfig(yamlPath, ""); err == nil {
			cfgYaml = *cfg
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
