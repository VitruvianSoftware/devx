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
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/ci"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	ciRunJob     []string
	ciRunImage   string
	ciRunRuntime string
)

var ciRunCmd = &cobra.Command{
	Use:   "run [workflow.yml]",
	Short: "Execute a GitHub Actions workflow locally",
	Long: `Parse and execute a GitHub Actions workflow file inside isolated containers.

This command provides ~80% parity with GitHub Actions for local debugging.
It executes all run: shell blocks inside Podman/Docker containers, resolves
strategy.matrix into parallel jobs, and respects needs: job dependencies.

LIMITATION: uses: actions (like actions/setup-go, golangci/golangci-lint-action,
or actions/upload-artifact) are intentionally SKIPPED with a visible warning.
Only run: shell blocks are executed. To replicate what a uses: action does,
add the equivalent shell commands to your workflow's run: blocks.

Examples:

  # Auto-discover and pick a workflow interactively
  devx ci run

  # Run a specific workflow
  devx ci run ci.yml

  # Run only the 'test' job
  devx ci run ci.yml --job test

  # Dry-run: show the execution plan without creating containers
  devx ci run ci.yml --dry-run

  # JSON output for AI agent consumption
  devx ci run ci.yml --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCIRun,
}

func init() {
	ciRunCmd.Flags().StringSliceVar(&ciRunJob, "job", nil,
		"Run only specific job(s) by name (comma-separated)")
	ciRunCmd.Flags().StringVar(&ciRunImage, "image", "",
		"Override the container image (default: auto-detect from devcontainer.json)")
	ciRunCmd.Flags().StringVar(&ciRunRuntime, "runtime", "podman",
		"Container runtime (podman or docker)")
	ciCmd.AddCommand(ciRunCmd)
}

func runCIRun(cmd *cobra.Command, args []string) error {
	projectDir, _ := filepath.Abs(".")

	var workflowPath string

	if len(args) > 0 {
		// User specified a workflow file
		input := args[0]
		// Allow shorthand: "ci.yml" → ".github/workflows/ci.yml"
		if !strings.Contains(input, "/") {
			input = filepath.Join(".github", "workflows", input)
		}
		workflowPath = input
	} else {
		// Discover workflows
		workflows, err := ci.DiscoverWorkflows(projectDir)
		if err != nil {
			return err
		}

		if len(workflows) == 1 || NonInteractive {
			workflowPath = workflows[0]
		} else {
			// Interactive picker
			options := make([]huh.Option[string], len(workflows))
			for i, w := range workflows {
				options[i] = huh.NewOption(filepath.Base(w), w)
			}
			err := huh.NewSelect[string]().
				Title("Select a workflow to run:").
				Options(options...).
				Value(&workflowPath).
				Run()
			if err != nil {
				return fmt.Errorf("workflow selection cancelled: %w", err)
			}
		}
	}

	// Parse the workflow
	wf, err := ci.ParseWorkflow(workflowPath)
	if err != nil {
		return err
	}

	wfName := wf.Name
	if wfName == "" {
		wfName = filepath.Base(workflowPath)
	}

	if !outputJSON {
		fmt.Printf("🏗️  devx ci run — %s\n", wfName)
		fmt.Printf("   Workflow: %s\n", workflowPath)
		fmt.Printf("   Runtime:  %s\n", ciRunRuntime)
		if len(ciRunJob) > 0 {
			fmt.Printf("   Filter:   %s\n", strings.Join(ciRunJob, ", "))
		}
		if DryRun {
			fmt.Println("   Mode:     dry-run")
		}
	}

	// Load secrets from devx vault (best-effort)
	secrets := loadVaultSecrets()

	cfg := ci.ExecuteConfig{
		Workflow:   wf,
		Runtime:    ciRunRuntime,
		Image:      ciRunImage,
		JobFilter:  ciRunJob,
		Secrets:    secrets,
		DryRun:     DryRun,
		JSONOutput: outputJSON,
		ProjectDir: projectDir,
	}

	_, err = ci.Execute(cfg)
	return err
}

// loadVaultSecrets attempts to load secrets from devx vault providers.
// Returns an empty map on failure — secrets are best-effort for local CI.
func loadVaultSecrets() map[string]string {
	// TODO: Wire into internal/envvault once the vault providers are
	// integrated. For now, return empty secrets.
	return map[string]string{}
}
