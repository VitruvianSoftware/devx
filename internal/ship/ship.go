// Package ship implements the deterministic agent pipeline guardrail.
// It wraps pre-flight checks, git push, PR creation, and synchronous
// CI polling into a single blocking operation so AI agents cannot
// skip post-merge verification.
package ship

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/telemetry"
)

// ExitCodes for deterministic agent error handling.
const (
	ExitOK             = 0
	ExitPreFlightFail  = 50
	ExitPushFail       = 51
	ExitPRFail         = 52
	ExitCIFail         = 53
	ExitCITimeout      = 54
	ExitDocCheckFail   = 55
	ExitNothingToShip  = 56
)

// Result is the machine-readable output of a ship operation.
type Result struct {
	Success     bool         `json:"success"`
	ExitCode    int          `json:"exit_code"`
	Phase       string       `json:"phase"`
	Message     string       `json:"message"`
	PRURL       string       `json:"pr_url,omitempty"`
	CIRunID     string       `json:"ci_run_id,omitempty"`
	CIStatus    string       `json:"ci_status,omitempty"`
	FailureLogs []string     `json:"failure_logs,omitempty"`
	PreFlight   *PreFlightResult `json:"pre_flight,omitempty"`
}

// PreFlightResult holds the outcome of each local pre-flight step.
type PreFlightResult struct {
	Stack    string `json:"stack"`
	TestPass bool   `json:"test_pass"`
	LintPass bool   `json:"lint_pass"`
	BuildPass bool  `json:"build_pass"`
	TestSkipped bool `json:"test_skipped,omitempty"`
	LintSkipped bool `json:"lint_skipped,omitempty"`
	BuildSkipped bool `json:"build_skipped,omitempty"`
}

// Options configures a ship run.
type Options struct {
	CommitMsg      string
	Branch         string  // target branch (default: current)
	BaseBranch     string  // base branch for PR (default: main)
	Verbose        bool
	JSON           bool
	NonInteractive bool
	SkipPreFlight  bool
	CITimeout      time.Duration
}

// stackInfo defines what commands to run for each detected stack.
type stackInfo struct {
	Name     string
	TestCmd  []string
	LintCmd  []string
	BuildCmd []string
}

// DetectStack profiles the repository by looking for marker files.
func DetectStack(dir string) *stackInfo {
	checks := []struct {
		marker string
		stack  stackInfo
	}{
		{"go.mod", stackInfo{
			Name:     "Go",
			TestCmd:  []string{"go", "test", "./..."},
			LintCmd:  []string{"go", "vet", "./..."},
			BuildCmd: []string{"go", "build", "./..."},
		}},
		{"package.json", stackInfo{
			Name:     "Node/JS/TS",
			TestCmd:  []string{"npm", "test"},
			LintCmd:  []string{"npm", "run", "lint"},
			BuildCmd: []string{"npm", "run", "build"},
		}},
		{"Cargo.toml", stackInfo{
			Name:     "Rust",
			TestCmd:  []string{"cargo", "test"},
			LintCmd:  []string{"cargo", "clippy"},
			BuildCmd: []string{"cargo", "build"},
		}},
		{"pyproject.toml", stackInfo{
			Name:     "Python",
			TestCmd:  []string{"pytest"},
			LintCmd:  []string{"ruff", "check", "."},
			BuildCmd: nil,
		}},
	}

	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c.marker)); err == nil {
			return &c.stack
		}
	}
	return nil
}

// RunPreFlight executes local tests, linter, and build for the detected stack.
func RunPreFlight(dir string, verbose bool) (*PreFlightResult, error) {
	stack := DetectStack(dir)
	if stack == nil {
		return &PreFlightResult{Stack: "unknown"}, nil
	}

	result := &PreFlightResult{Stack: stack.Name}

	// Tests
	if len(stack.TestCmd) > 0 {
		if err := runCmd(dir, stack.TestCmd, verbose); err != nil {
			// For Node projects, skip if no test script
			if stack.Name == "Node/JS/TS" && strings.Contains(err.Error(), "Missing script") {
				result.TestSkipped = true
				result.TestPass = true
			} else {
				return result, fmt.Errorf("tests failed: %w", err)
			}
		} else {
			result.TestPass = true
		}
	} else {
		result.TestSkipped = true
		result.TestPass = true
	}

	// Lint
	if len(stack.LintCmd) > 0 {
		if err := runCmd(dir, stack.LintCmd, verbose); err != nil {
			if stack.Name == "Node/JS/TS" && strings.Contains(err.Error(), "Missing script") {
				result.LintSkipped = true
				result.LintPass = true
			} else {
				return result, fmt.Errorf("linter failed: %w", err)
			}
		} else {
			result.LintPass = true
		}
	} else {
		result.LintSkipped = true
		result.LintPass = true
	}

	// Build
	if len(stack.BuildCmd) > 0 {
		buildStart := time.Now()
		if err := runCmd(dir, stack.BuildCmd, verbose); err != nil {
			if stack.Name == "Node/JS/TS" && strings.Contains(err.Error(), "Missing script") {
				result.BuildSkipped = true
				result.BuildPass = true
			} else {
				return result, fmt.Errorf("build failed: %w", err)
			}
		} else {
			result.BuildPass = true
		}
		buildDur := time.Since(buildStart)
		telemetry.RecordEvent("agent_ship_build", buildDur)
		telemetry.NudgeIfSlow("build", buildDur, 60*time.Second, false)
	} else {
		result.BuildSkipped = true
		result.BuildPass = true
	}

	return result, nil
}

// GitPush stages, commits, and pushes to a feature branch using --no-verify
// to bypass our own pre-push hook.
func GitPush(dir, commitMsg, branch string) error {
	// Stage all changes
	if err := runCmd(dir, []string{"git", "add", "-A"}, false); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Commit
	if err := runCmd(dir, []string{"git", "commit", "-m", commitMsg}, false); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	// Push with --no-verify to bypass our own pre-push hook
	args := []string{"git", "push", "--no-verify"}
	if branch != "" {
		args = append(args, "-u", "origin", branch)
	}
	if err := runCmd(dir, args, false); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// CreateAndMergePR creates a GitHub PR and merges it.
// Returns the PR URL.
func CreateAndMergePR(dir, title, body, baseBranch string) (string, error) {
	// Create PR
	out, err := runCmdOutput(dir, []string{
		"gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--base", baseBranch,
	})
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w", err)
	}
	prURL := strings.TrimSpace(out)

	// Merge PR
	if err := runCmd(dir, []string{
		"gh", "pr", "merge", prURL,
		"--squash", "--admin", "-d",
	}, false); err != nil {
		return prURL, fmt.Errorf("gh pr merge: %w", err)
	}

	return prURL, nil
}

// PollCI waits for the latest CI run on the given branch to complete.
// It blocks until the pipeline finishes or the timeout expires.
// Returns the run conclusion and any failure logs.
func PollCI(dir, branch string, timeout time.Duration) (runID, conclusion string, failureLogs []string, err error) {
	deadline := time.Now().Add(timeout)

	// Wait a few seconds for GitHub to register the push
	time.Sleep(10 * time.Second)

	for time.Now().Before(deadline) {
		// Get the latest run ID
		out, cmdErr := runCmdOutput(dir, []string{
			"gh", "run", "list",
			"--branch", branch,
			"-L", "1",
			"--json", "databaseId,status,conclusion",
		})
		if cmdErr != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		var runs []struct {
			DatabaseID int64  `json:"databaseId"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		}
		if jsonErr := json.Unmarshal([]byte(out), &runs); jsonErr != nil || len(runs) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		run := runs[0]
		runID = fmt.Sprintf("%d", run.DatabaseID)

		if run.Status == "completed" {
			conclusion = run.Conclusion
			if conclusion != "success" {
				// Fetch failure logs
				logOut, logErr := runCmdOutput(dir, []string{
					"gh", "run", "view", runID, "--log-failed",
				})
				if logErr == nil {
					failureLogs = condenseFailureLogs(logOut)
				}
			}
			return runID, conclusion, failureLogs, nil
		}

		time.Sleep(10 * time.Second)
	}

	return runID, "timeout", nil, fmt.Errorf("CI pipeline did not complete within %s", timeout)
}

// condenseFailureLogs strips verbose CI noise (setup steps, cache hits, etc.)
// and returns only actionable error lines.
func condenseFailureLogs(raw string) []string {
	var condensed []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip common CI noise
		lower := strings.ToLower(line)
		if strings.Contains(lower, "cache hit") ||
			strings.Contains(lower, "cache restored") ||
			strings.Contains(lower, "##[group]") ||
			strings.Contains(lower, "##[endgroup]") ||
			strings.Contains(lower, "downloading") ||
			strings.Contains(lower, "installed ") ||
			strings.Contains(lower, "unpacking") ||
			strings.Contains(lower, "remote: enumerating") ||
			strings.Contains(lower, "remote: counting") ||
			strings.Contains(lower, "remote: compressing") {
			continue
		}
		condensed = append(condensed, line)
	}
	// Cap at 50 lines to keep output agent-friendly
	if len(condensed) > 50 {
		condensed = condensed[:50]
	}
	return condensed
}

// HasStagedChanges checks if there are any uncommitted changes.
func HasStagedChanges(dir string) bool {
	out, err := runCmdOutput(dir, []string{"git", "status", "--porcelain"})
	if err != nil {
		return false
	}
	return strings.TrimSpace(out) != ""
}

// CurrentBranch returns the current git branch name.
func CurrentBranch(dir string) string {
	out, err := runCmdOutput(dir, []string{"git", "rev-parse", "--abbrev-ref", "HEAD"})
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func runCmd(dir string, args []string, verbose bool) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = nil
		cmd.Stderr = nil
	}
	return cmd.Run()
}

func runCmdOutput(dir string, args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
