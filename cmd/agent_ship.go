package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

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

	if shipCommitMsg == "" {
		return fmt.Errorf("commit message is required: devx agent ship -m \"your message\"")
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
		if cfg, err := resolveConfig("devx.yaml", ""); err == nil && cfg.Pipeline != nil {
			pipeline = convertPipeline(cfg.Pipeline)
			if !outputJSON {
				fmt.Printf("    %s  using devx.yaml pipeline config\n", shipStyleMuted.Render("ℹ"))
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
		"Commit message (required)")
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
