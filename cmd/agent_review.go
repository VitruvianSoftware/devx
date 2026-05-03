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
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/ship"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var (
	reviewCommitMsg     string
	reviewBaseBranch    string
	reviewCITimeout     time.Duration
	reviewSkipPreflight bool
	reviewVerbose       bool
)

var agentReviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Commit, push, and create a PR for human review",
	Long: `The deterministic agent pipeline guardrail for human review workflows.
Wraps the entire commit → push → PR → CI verification lifecycle into one
command that blocks until the CI pipeline completes on the target branch.

Unlike 'ship', this command does NOT auto-merge the PR. It leaves it open
for human review.

  devx agent review -m "feat: add new feature"

This command:
  1. Runs local pre-flight checks (test, lint, build) for the detected stack
  2. Commits and pushes the changes (bypassing the pre-push hook internally)
  3. Creates a PR
  4. Polls the CI pipeline and BLOCKS until it completes
  5. Reports the final result with deterministic exit codes

Exit codes:
  0   — Success: all checks passed, CI is green, PR is open
  50  — Pre-flight failure (tests, lint, or build failed locally)
  51  — Git push failed
  52  — PR creation failed
  53  — CI pipeline failed (failure logs included in output)
  54  — CI pipeline timed out
  56  — Nothing to review (no changes detected)

Machine-readable output:
  devx agent review -m "fix: resolve bug" --json`,
	RunE: runAgentReview,
}

func runAgentReview(_ *cobra.Command, _ []string) error {
	if reviewCommitMsg == "" {
		return fmt.Errorf("commit message is required: devx agent review -m \"your message\"")
	}

	cwd, _ := os.Getwd()
	result := &ship.Result{Phase: "init"}

	// ── Phase 0: Check for changes ──────────────────────────────────────
	if !ship.HasStagedChanges(cwd) {
		result.ExitCode = ship.ExitNothingToShip
		result.Phase = "check"
		result.Message = "nothing to review — no uncommitted changes detected"
		return exitWithResult(result)
	}

	branch := ship.CurrentBranch(cwd)
	if reviewBaseBranch == "" {
		reviewBaseBranch = "main"
	}

	if !outputJSON {
		fmt.Println()
		fmt.Println(tui.StyleTitle.Render("🔍 devx agent review"))
		fmt.Println()
	}

	// ── Phase 1: Pre-flight checks ──────────────────────────────────────
	if !reviewSkipPreflight {
		if !outputJSON {
			fmt.Printf("  %s %s\n", shipStylePhase.Render("▸ Phase 1:"), "Pre-flight checks")
		}

		var pipeline *ship.PipelineConfig
		if yamlPath, err := findDevxConfig(); err == nil {
			if cfg, err := resolveConfig(yamlPath, ""); err == nil && cfg.Pipeline != nil {
				pipeline = convertPipeline(cfg.Pipeline)
				if !outputJSON {
					fmt.Printf("    %s  using devx.yaml pipeline config\n", shipStyleMuted.Render("ℹ"))
				}
			}
		}

		pfResult, err := ship.RunPreFlight(cwd, reviewVerbose, pipeline)
		result.PreFlight = pfResult

		if err != nil {
			result.ExitCode = ship.ExitPreFlightFail
			result.Phase = "pre-flight"
			result.Message = err.Error()
			if !outputJSON {
				fmt.Printf("    %s  %s\n\n", shipStyleFail.Render("✗ FAIL"), err.Error())
			}
			return exitWithResult(result)
		}

		if !outputJSON {
			fmt.Printf("    %s  %s (%s)\n",
				shipStylePass.Render("✓ PASS"),
				"all local checks passed",
				shipStyleMuted.Render(pfResult.Stack),
			)
		}
	}

	// ── Phase 2: Commit & Push ──────────────────────────────────────────
	if !outputJSON {
		fmt.Printf("  %s %s\n", shipStylePhase.Render("▸ Phase 2:"), "Commit & Push")
	}

	if err := ship.GitPush(cwd, reviewCommitMsg, branch); err != nil {
		result.ExitCode = ship.ExitPushFail
		result.Phase = "push"
		result.Message = err.Error()
		if !outputJSON {
			fmt.Printf("    %s  %s\n\n", shipStyleFail.Render("✗ FAIL"), err.Error())
		}
		return exitWithResult(result)
	}

	if !outputJSON {
		fmt.Printf("    %s  pushed to %s\n",
			shipStylePass.Render("✓"),
			shipStyleMuted.Render(branch),
		)
	}

	// ── Phase 3: Create PR for Review ───────────────────────────────────
	if !outputJSON {
		fmt.Printf("  %s %s\n", shipStylePhase.Render("▸ Phase 3:"), "Create PR for Review")
	}

	prURL, err := ship.CreatePR(cwd, reviewCommitMsg, reviewCommitMsg, reviewBaseBranch)
	result.PRURL = prURL
	if err != nil {
		result.ExitCode = ship.ExitPRFail
		result.Phase = "pr"
		result.Message = err.Error()
		if !outputJSON {
			fmt.Printf("    %s  %s\n\n", shipStyleFail.Render("✗ FAIL"), err.Error())
		}
		return exitWithResult(result)
	}

	if !outputJSON {
		fmt.Printf("    %s  %s\n",
			shipStylePass.Render("✓"),
			shipStyleMuted.Render(prURL),
		)
	}

	// ── Phase 4: Synchronous CI Polling (The Wait Trap) ─────────────────
	if !outputJSON {
		fmt.Println()
		fmt.Printf("  %s %s\n",
			shipStyleBlocking.Render("▸ Phase 4:"),
			shipStyleBlocking.Render("Waiting for CI pipeline..."),
		)
		fmt.Printf("    %s\n",
			shipStyleMuted.Render("⏳ Terminal is blocked until CI completes. Do not interrupt."),
		)
	}

	runID, conclusion, failureLogs, pollErr := ship.WatchPRChecks(cwd, prURL, branch, reviewCITimeout)
	result.CIRunID = runID
	result.CIStatus = conclusion
	result.FailureLogs = failureLogs

	if pollErr != nil {
		result.ExitCode = ship.ExitCITimeout
		result.Phase = "ci-poll"
		result.Message = pollErr.Error()
		if !outputJSON {
			fmt.Printf("    %s  %s\n\n", shipStyleFail.Render("✗ TIMEOUT"), pollErr.Error())
		}
		return exitWithResult(result)
	}

	if conclusion != "success" {
		result.ExitCode = ship.ExitCIFail
		result.Phase = "ci"
		result.Message = fmt.Sprintf("CI pipeline failed with conclusion: %s", conclusion)
		if !outputJSON {
			fmt.Printf("    %s  CI pipeline %s (run %s)\n",
				shipStyleFail.Render("✗ FAIL"),
				conclusion,
				runID,
			)
			if len(failureLogs) > 0 {
				fmt.Println()
				fmt.Println(shipStylePhase.Render("  ── Failure Logs (condensed) ──"))
				for _, line := range failureLogs {
					fmt.Printf("    %s\n", line)
				}
			}
			fmt.Println()
		}
		return exitWithResult(result)
	}

	// ── Success ─────────────────────────────────────────────────────────
	result.Success = true
	result.Phase = "complete"
	result.Message = "review PR created successfully — CI is green"

	if !outputJSON {
		fmt.Printf("    %s  CI pipeline %s (run %s)\n",
			shipStylePass.Render("✓ GREEN"),
			shipStylePass.Render("passed"),
			shipStyleMuted.Render(runID),
		)
		fmt.Println()
		fmt.Println(tui.StyleSuccessBox.Render("✅ Review PR is ready! CI is green. Waiting for human review."))
		fmt.Println()
	}

	return exitWithResult(result)
}

func init() {
	agentReviewCmd.Flags().StringVarP(&reviewCommitMsg, "message", "m", "",
		"Commit message (required)")
	agentReviewCmd.Flags().StringVar(&reviewBaseBranch, "base", "main",
		"Base branch for the PR (default: main)")
	agentReviewCmd.Flags().DurationVar(&reviewCITimeout, "ci-timeout", 10*time.Minute,
		"Maximum time to wait for CI pipeline completion")
	agentReviewCmd.Flags().BoolVar(&reviewSkipPreflight, "skip-preflight", false,
		"Skip local pre-flight checks (not recommended)")
	agentReviewCmd.Flags().BoolVarP(&reviewVerbose, "verbose", "v", false,
		"Show full output from pre-flight commands")

	agentCmd.AddCommand(agentReviewCmd)
}
