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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/ai"
	"github.com/VitruvianSoftware/devx/internal/ship"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var (
	shipCommitMsg     string
	shipBaseBranch    string
	shipCITimeout     time.Duration
	shipSkipPreflight bool
	shipInstallHook   bool
	shipVerbose       bool
)

var agentShipCmd = &cobra.Command{
	Use:   "ship",
	Short: "Commit, push, and verify CI in a single blocking operation",
	Long: `The deterministic agent pipeline guardrail. Wraps the entire
commit → push → PR → CI verification lifecycle into one command that
blocks until the CI pipeline completes on the target branch.

  devx agent ship -m "feat: add new feature"

This command:
  1. Runs local pre-flight checks (test, lint, build) for the detected stack
  2. Commits and pushes the changes (bypassing the pre-push hook internally)
  3. Creates a PR and merges it
  4. Polls the CI pipeline and BLOCKS until it completes
  5. Reports the final result with deterministic exit codes

Exit codes:
  0   — Success: all checks passed, CI is green
  50  — Pre-flight failure (tests, lint, or build failed locally)
  51  — Git push failed
  52  — PR creation or merge failed
  53  — CI pipeline failed (failure logs included in output)
  54  — CI pipeline timed out
  55  — Documentation check failed
  56  — Nothing to ship (no changes detected)

Humans can also use this command if they want the same CI-blocking workflow.

Machine-readable output:
  devx agent ship -m "fix: resolve bug" --json`,
	RunE: runAgentShip,
}

// ── Styles ──────────────────────────────────────────────────────────────────

var (
	shipStylePhase    = lipgloss.NewStyle().Foreground(lipgloss.Color("#79C0FF")).Bold(true)
	shipStylePass     = lipgloss.NewStyle().Foreground(lipgloss.Color("#3FB950")).Bold(true)
	shipStyleFail     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF7B72")).Bold(true)
	shipStyleMuted    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B949E"))
	shipStyleBlocking = lipgloss.NewStyle().Foreground(lipgloss.Color("#E3B341")).Bold(true)
)

func runAgentShip(_ *cobra.Command, _ []string) error {
	// Handle --install-hook mode
	if shipInstallHook {
		return installShipHook()
	}

	// Auto-generate commit message via AI if -m is not provided
	if shipCommitMsg == "" {
		generated, err := generateCommitMessage()
		if err != nil {
			return fmt.Errorf("commit message is required: devx agent ship -m \"your message\"\n  (auto-generate failed: %v)", err)
		}
		shipCommitMsg = generated
	}

	cwd, _ := os.Getwd()
	result := &ship.Result{Phase: "init"}

	// ── Phase 0: Check for changes ──────────────────────────────────────
	if !ship.HasStagedChanges(cwd) {
		result.ExitCode = ship.ExitNothingToShip
		result.Phase = "check"
		result.Message = "nothing to ship — no uncommitted changes detected"
		return exitWithResult(result)
	}

	branch := ship.CurrentBranch(cwd)
	if shipBaseBranch == "" {
		shipBaseBranch = "main"
	}

	if !outputJSON {
		fmt.Println()
		fmt.Println(tui.StyleTitle.Render("🚀 devx agent ship"))
		fmt.Println()
	}

	// ── Phase 1: Pre-flight checks ──────────────────────────────────────
	if !shipSkipPreflight {
		if !outputJSON {
			fmt.Printf("  %s %s\n", shipStylePhase.Render("▸ Phase 1:"), "Pre-flight checks")
		}

		// Load explicit pipeline from devx.yaml if present
		var pipeline *ship.PipelineConfig
		if yamlPath, err := findDevxConfig(); err == nil {
			if cfg, err := resolveConfig(yamlPath, ""); err == nil && cfg.Pipeline != nil {
				pipeline = convertPipeline(cfg.Pipeline)
				if !outputJSON {
					fmt.Printf("    %s  using devx.yaml pipeline config\n", shipStyleMuted.Render("ℹ"))
				}
			}
		}

		pfResult, err := ship.RunPreFlight(cwd, shipVerbose, pipeline)
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

	if err := ship.GitPush(cwd, shipCommitMsg, branch); err != nil {
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

	// ── Phase 3: Create PR ──────────────────────────────────────────────
	if !outputJSON {
		fmt.Printf("  %s %s\n", shipStylePhase.Render("▸ Phase 3:"), "Create PR")
	}

	prURL, err := ship.CreatePR(cwd, shipCommitMsg, shipCommitMsg, shipBaseBranch)
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

	runID, conclusion, failureLogs, pollErr := ship.WatchPRChecks(cwd, prURL, branch, shipCITimeout)
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

	// ── Phase 5: Merge PR ───────────────────────────────────────────────
	if !outputJSON {
		fmt.Printf("  %s %s\n", shipStylePhase.Render("▸ Phase 5:"), "Merge PR")
	}

	if err := ship.MergePR(cwd, prURL); err != nil {
		result.ExitCode = ship.ExitPRFail
		result.Phase = "merge"
		result.Message = err.Error()
		if !outputJSON {
			fmt.Printf("    %s  %s\n\n", shipStyleFail.Render("✗ FAIL"), err.Error())
		}
		return exitWithResult(result)
	}

	// ── Success ─────────────────────────────────────────────────────────
	result.Success = true
	result.Phase = "complete"
	result.Message = "shipped successfully — CI is green"

	if !outputJSON {
		fmt.Printf("    %s  CI pipeline %s (run %s)\n",
			shipStylePass.Render("✓ GREEN"),
			shipStylePass.Render("passed"),
			shipStyleMuted.Render(runID),
		)
		fmt.Println()
		fmt.Println(tui.StyleSuccessBox.Render("✅ Ship complete! CI is green and code is merged."))
		fmt.Println()
	}

	return exitWithResult(result)
}

func exitWithResult(r *ship.Result) error {
	if outputJSON {
		enc, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(enc))
	}

	if r.ExitCode != ship.ExitOK {
		os.Exit(r.ExitCode)
	}
	return nil
}

func installShipHook() error {
	cwd, _ := os.Getwd()

	if err := ship.InstallPrePushHook(cwd); err != nil {
		if strings.Contains(err.Error(), "non-devx") {
			fmt.Printf("  %s  %s\n", shipStyleFail.Render("✗"), err.Error())
			return nil
		}
		return err
	}

	fmt.Printf("  %s  Installed devx pre-push hook at .git/hooks/pre-push\n", tui.IconDone)
	fmt.Printf("  %s\n", shipStyleMuted.Render("Direct 'git push' is now blocked. AI agents must use 'devx agent ship'."))
	fmt.Printf("  %s\n", shipStyleMuted.Render("Humans can bypass with: git push --no-verify"))
	return nil
}

func init() {
	agentShipCmd.Flags().StringVarP(&shipCommitMsg, "message", "m", "",
		"Commit message (omit to auto-generate via AI)")
	agentShipCmd.Flags().StringVar(&shipBaseBranch, "base", "main",
		"Base branch for the PR (default: main)")
	agentShipCmd.Flags().DurationVar(&shipCITimeout, "ci-timeout", 10*time.Minute,
		"Maximum time to wait for CI pipeline completion")
	agentShipCmd.Flags().BoolVar(&shipSkipPreflight, "skip-preflight", false,
		"Skip local pre-flight checks (not recommended)")
	agentShipCmd.Flags().BoolVar(&shipInstallHook, "install-hook", false,
		"Install the pre-push git hook and exit")
	agentShipCmd.Flags().BoolVarP(&shipVerbose, "verbose", "v", false,
		"Show full output from pre-flight commands")

	agentCmd.AddCommand(agentShipCmd)
}

// convertPipeline bridges the devx.yaml config model to the ship package's PipelineConfig.
func convertPipeline(p *DevxConfigPipeline) *ship.PipelineConfig {
	if p == nil {
		return nil
	}
	return &ship.PipelineConfig{
		Test:   convertStage(p.Test),
		Lint:   convertStage(p.Lint),
		Build:  convertStage(p.Build),
		Verify: convertStage(p.Verify),
	}
}

func convertStage(s *DevxConfigPipelineStage) *ship.PipelineStage {
	if s == nil {
		return nil
	}
	cmds := s.Cmds()
	if len(cmds) == 0 && len(s.Before) == 0 && len(s.After) == 0 {
		return nil
	}
	return &ship.PipelineStage{
		Cmds:   cmds,
		Before: s.Before,
		After:  s.After,
	}
}

// generateCommitMessage uses AI to auto-generate a conventional commit message
// from the current git diff.
func generateCommitMessage() (string, error) {
	// Get the diff
	diff, err := exec.Command("git", "diff", "--cached", "--stat").CombinedOutput()
	if err != nil || strings.TrimSpace(string(diff)) == "" {
		// No staged changes — try unstaged
		diff, err = exec.Command("git", "diff", "--stat").CombinedOutput()
		if err != nil || strings.TrimSpace(string(diff)) == "" {
			return "", fmt.Errorf("no diff available to generate commit message from")
		}
	}

	// Get a more detailed diff for the AI (limited to avoid token overflow)
	detailedDiff, _ := exec.Command("git", "diff", "--cached").CombinedOutput()
	if strings.TrimSpace(string(detailedDiff)) == "" {
		detailedDiff, _ = exec.Command("git", "diff").CombinedOutput()
	}

	// Truncate diff if too large (keep first 4000 chars)
	diffText := string(detailedDiff)
	if len(diffText) > 4000 {
		diffText = diffText[:4000] + "\n... (diff truncated)"
	}

	prompt := fmt.Sprintf(`Generate a conventional commit message for this diff. Follow these rules:
1. Use the format: type(scope): description
2. Types: feat, fix, docs, refactor, test, chore, style, perf
3. Keep the first line under 72 characters
4. Add a blank line then a brief body if the change is complex
5. Output ONLY the commit message text — no explanations, no code fences, no quotes

Diff:
%s`, diffText)

	if !outputJSON {
		fmt.Printf("  %s %s\n", shipStylePhase.Render("▸"), "Auto-generating commit message via AI...")
	}

	result, err := ai.RunAgentPrompt(prompt)
	if err != nil {
		return "", err
	}

	if result.Mode == ai.AgentModeNone {
		return "", fmt.Errorf("no AI provider available")
	}

	msg := strings.TrimSpace(result.Output)
	if msg == "" {
		return "", fmt.Errorf("AI returned empty commit message")
	}

	// Clean up: remove any wrapping quotes the AI might add
	msg = strings.Trim(msg, "\"'`")

	if !outputJSON {
		fmt.Printf("    %s  %s %s\n",
			shipStylePass.Render("✓"),
			"generated via",
			shipStyleMuted.Render(string(result.Mode)),
		)
		fmt.Printf("    %s  %s\n\n",
			shipStyleMuted.Render("→"),
			msg,
		)
	}

	return msg, nil
}
