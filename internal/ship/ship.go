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

// Package ship implements the deterministic agent pipeline guardrail.
// It wraps pre-flight checks, git push, PR creation, and synchronous
// CI polling into a single blocking operation so AI agents cannot
// skip post-merge verification.
package ship

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/telemetry"
)

// ErrNoStack is returned when no stack is detected and no explicit pipeline exists.
// Callers should surface this as a warning to the developer.
var ErrNoStack = errors.New("no recognized stack detected (go.mod, package.json, Cargo.toml, pyproject.toml, pom.xml, build.gradle, *.csproj) and no pipeline: block in devx.yaml — pre-flight checks were skipped")

// ExitCodes for deterministic agent error handling.
const (
	ExitOK            = 0
	ExitPreFlightFail = 50
	ExitPushFail      = 51
	ExitPRFail        = 52
	ExitCIFail        = 53
	ExitCITimeout     = 54
	ExitDocCheckFail  = 55
	ExitNothingToShip = 56
)

// Result is the machine-readable output of a ship operation.
type Result struct {
	Success     bool             `json:"success"`
	ExitCode    int              `json:"exit_code"`
	Phase       string           `json:"phase"`
	Message     string           `json:"message"`
	PRURL       string           `json:"pr_url,omitempty"`
	CIRunID     string           `json:"ci_run_id,omitempty"`
	CIStatus    string           `json:"ci_status,omitempty"`
	FailureLogs []string         `json:"failure_logs,omitempty"`
	PreFlight   *PreFlightResult `json:"pre_flight,omitempty"`
}

// PreFlightResult holds the outcome of each local pre-flight step.
type PreFlightResult struct {
	Stack        string `json:"stack"`
	TestPass     bool   `json:"test_pass"`
	LintPass     bool   `json:"lint_pass"`
	BuildPass    bool   `json:"build_pass"`
	TestSkipped  bool   `json:"test_skipped,omitempty"`
	LintSkipped  bool   `json:"lint_skipped,omitempty"`
	BuildSkipped bool   `json:"build_skipped,omitempty"`
}

// Options configures a ship run.
type Options struct {
	CommitMsg      string
	Branch         string // target branch (default: current)
	BaseBranch     string // base branch for PR (default: main)
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

// stackDefinitions returns all supported stack detection rules.
// The order determines precedence when multiple markers are found.
func stackDefinitions() []struct {
	marker string
	glob   bool // if true, use filepath.Glob instead of os.Stat
	stack  stackInfo
} {
	return []struct {
		marker string
		glob   bool
		stack  stackInfo
	}{
		{"go.mod", false, stackInfo{
			Name:     "Go",
			TestCmd:  []string{"go", "test", "./..."},
			LintCmd:  []string{"go", "vet", "./..."},
			BuildCmd: []string{"go", "build", "./..."},
		}},
		{"package.json", false, stackInfo{
			Name:     "Node/JS/TS",
			TestCmd:  []string{"npm", "test"},
			LintCmd:  []string{"npm", "run", "lint"},
			BuildCmd: []string{"npm", "run", "build"},
		}},
		{"Cargo.toml", false, stackInfo{
			Name:     "Rust",
			TestCmd:  []string{"cargo", "test"},
			LintCmd:  []string{"cargo", "clippy"},
			BuildCmd: []string{"cargo", "build"},
		}},
		{"pyproject.toml", false, stackInfo{
			Name:     "Python",
			TestCmd:  []string{"pytest"},
			LintCmd:  []string{"ruff", "check", "."},
			BuildCmd: nil,
		}},
		{"pom.xml", false, stackInfo{
			Name:     "Java/Maven",
			TestCmd:  []string{"mvn", "test"},
			LintCmd:  []string{"mvn", "checkstyle:check"},
			BuildCmd: []string{"mvn", "package", "-DskipTests"},
		}},
		{"build.gradle", false, stackInfo{
			Name:     "Java/Gradle",
			TestCmd:  []string{"./gradlew", "test"},
			LintCmd:  []string{"./gradlew", "check"},
			BuildCmd: []string{"./gradlew", "build", "-x", "test"},
		}},
		{"build.gradle.kts", false, stackInfo{
			Name:     "Kotlin/Gradle",
			TestCmd:  []string{"./gradlew", "test"},
			LintCmd:  []string{"./gradlew", "check"},
			BuildCmd: []string{"./gradlew", "build", "-x", "test"},
		}},
		{"*.csproj", true, stackInfo{
			Name:     ".NET",
			TestCmd:  []string{"dotnet", "test"},
			LintCmd:  []string{"dotnet", "format", "--verify-no-changes"},
			BuildCmd: []string{"dotnet", "build"},
		}},
		{"*.sln", true, stackInfo{
			Name:     ".NET",
			TestCmd:  []string{"dotnet", "test"},
			LintCmd:  []string{"dotnet", "format", "--verify-no-changes"},
			BuildCmd: []string{"dotnet", "build"},
		}},
		{"MODULE.bazel", false, stackInfo{
			Name:     "Bazel",
			TestCmd:  []string{"bazel", "test", "//..."},
			LintCmd:  nil,
			BuildCmd: []string{"bazel", "build", "//..."},
		}},
		{"WORKSPACE", false, stackInfo{
			Name:     "Bazel",
			TestCmd:  []string{"bazel", "test", "//..."},
			LintCmd:  nil,
			BuildCmd: []string{"bazel", "build", "//..."},
		}},
		{"WORKSPACE.bazel", false, stackInfo{
			Name:     "Bazel",
			TestCmd:  []string{"bazel", "test", "//..."},
			LintCmd:  nil,
			BuildCmd: []string{"bazel", "build", "//..."},
		}},
	}
}

// DetectStack profiles the repository by looking for marker files.
// Returns the first matching stack (highest precedence).
func DetectStack(dir string) *stackInfo {
	stacks := DetectAllStacks(dir)
	if len(stacks) == 0 {
		return nil
	}
	return stacks[0]
}

// DetectAllStacks returns all matching stacks for the given directory.
// This enables multi-stack monorepo support.
func DetectAllStacks(dir string) []*stackInfo {
	var matched []*stackInfo
	seen := make(map[string]bool)

	for _, c := range stackDefinitions() {
		if seen[c.stack.Name] {
			continue
		}
		var found bool
		if c.glob {
			matches, _ := filepath.Glob(filepath.Join(dir, c.marker))
			found = len(matches) > 0
		} else {
			_, err := os.Stat(filepath.Join(dir, c.marker))
			found = err == nil
		}
		if found {
			s := c.stack // copy
			matched = append(matched, &s)
			seen[c.stack.Name] = true
		}
	}
	return matched
}

// PipelineStage defines a single pipeline step with support for multi-command
// and lifecycle hooks (before/after).
type PipelineStage struct {
	Cmds   [][]string // Resolved commands to run sequentially
	Before [][]string // Pre-stage hooks (run before Cmds)
	After  [][]string // Post-stage hooks (run after Cmds)
}

// PipelineConfig holds explicit pipeline stage overrides from devx.yaml.
// When non-nil, auto-detection via DetectStack is bypassed entirely ("Explicit Wins").
type PipelineConfig struct {
	Test   *PipelineStage
	Lint   *PipelineStage
	Build  *PipelineStage
	Verify *PipelineStage
}

// RunPreFlight executes local tests, linter, and build.
// If an explicit pipeline is provided, it takes precedence over auto-detection.
func RunPreFlight(dir string, verbose bool, pipeline *PipelineConfig) (*PreFlightResult, error) {
	if pipeline != nil {
		return runExplicitPipeline(dir, verbose, pipeline)
	}
	return runAutoDetectedPipeline(dir, verbose)
}

// runStageWithHooks executes before → cmds → after for a single pipeline stage.
// If the stage is nil or has no commands, it returns nil (no-op).
// Fail-fast: if a before hook fails, main commands and after hooks are skipped.
func runStageWithHooks(dir string, stage *PipelineStage, name string, verbose bool) error {
	if stage == nil || len(stage.Cmds) == 0 {
		return nil
	}
	for _, hook := range stage.Before {
		if err := runCmd(dir, hook, verbose); err != nil {
			return fmt.Errorf("%s before hook failed: %w", name, err)
		}
	}
	for _, cmd := range stage.Cmds {
		if err := runCmd(dir, cmd, verbose); err != nil {
			return fmt.Errorf("%s failed: %w", name, err)
		}
	}
	for _, hook := range stage.After {
		if err := runCmd(dir, hook, verbose); err != nil {
			return fmt.Errorf("%s after hook failed: %w", name, err)
		}
	}
	return nil
}

// runExplicitPipeline executes pipeline stages from devx.yaml config.
// Order: Lint → Test → Build → Verify (lint first to catch cheap issues
// before test side-effects can pollute the filesystem).
func runExplicitPipeline(dir string, verbose bool, pipeline *PipelineConfig) (*PreFlightResult, error) {
	preflightStart := time.Now()
	result := &PreFlightResult{Stack: "pipeline"}

	// Lint (runs first — cheap, no side effects)
	if pipeline.Lint != nil && len(pipeline.Lint.Cmds) > 0 {
		if err := runStageWithHooks(dir, pipeline.Lint, "lint", verbose); err != nil {
			return result, err
		}
		result.LintPass = true
	} else {
		result.LintSkipped = true
		result.LintPass = true
	}

	// Test
	if pipeline.Test != nil && len(pipeline.Test.Cmds) > 0 {
		if err := runStageWithHooks(dir, pipeline.Test, "test", verbose); err != nil {
			return result, err
		}
		result.TestPass = true
	} else {
		result.TestSkipped = true
		result.TestPass = true
	}

	// Build
	if pipeline.Build != nil && len(pipeline.Build.Cmds) > 0 {
		buildStart := time.Now()
		if err := runStageWithHooks(dir, pipeline.Build, "build", verbose); err != nil {
			return result, err
		}
		result.BuildPass = true
		buildDur := time.Since(buildStart)
		telemetry.RecordEvent("agent_ship_build", buildDur)
		telemetry.NudgeIfSlow("build", buildDur, 60*time.Second, false)
	} else {
		result.BuildSkipped = true
		result.BuildPass = true
	}

	// Verify (pipeline only)
	if err := runStageWithHooks(dir, pipeline.Verify, "verify", verbose); err != nil {
		return result, err
	}

	// Record enriched preflight span
	preflightDur := time.Since(preflightStart)
	telemetry.RecordEvent("agent_ship_preflight", preflightDur,
		telemetry.Attr("devx.stack", "pipeline"),
		telemetry.Attr("devx.pipeline", true),
		telemetry.Attr("devx.test.pass", result.TestPass),
		telemetry.Attr("devx.test.skipped", result.TestSkipped),
		telemetry.Attr("devx.lint.pass", result.LintPass),
		telemetry.Attr("devx.lint.skipped", result.LintSkipped),
		telemetry.Attr("devx.build.pass", result.BuildPass),
		telemetry.Attr("devx.build.skipped", result.BuildSkipped),
		telemetry.Attr("devx.project", filepath.Base(dir)),
		telemetry.Attr("devx.branch", currentBranch(dir)),
	)

	return result, nil
}

// runAutoDetectedPipeline executes the existing auto-detection logic.
func runAutoDetectedPipeline(dir string, verbose bool) (*PreFlightResult, error) {
	preflightStart := time.Now()

	stack := DetectStack(dir)
	if stack == nil {
		return &PreFlightResult{Stack: "unknown"}, ErrNoStack
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

	// Record enriched preflight span with full outcomes
	preflightDur := time.Since(preflightStart)
	telemetry.RecordEvent("agent_ship_preflight", preflightDur,
		telemetry.Attr("devx.stack", stack.Name),
		telemetry.Attr("devx.test.pass", result.TestPass),
		telemetry.Attr("devx.test.skipped", result.TestSkipped),
		telemetry.Attr("devx.lint.pass", result.LintPass),
		telemetry.Attr("devx.lint.skipped", result.LintSkipped),
		telemetry.Attr("devx.build.pass", result.BuildPass),
		telemetry.Attr("devx.build.skipped", result.BuildSkipped),
		telemetry.Attr("devx.project", filepath.Base(dir)),
		telemetry.Attr("devx.branch", currentBranch(dir)),
	)

	return result, nil
}

// commitAll stages and commits all working-tree changes if there are any. It is
// a no-op on a clean tree, so a branch whose work is already committed is pushed
// as-is rather than getting an empty/duplicate commit.
func commitAll(dir, commitMsg string) error {
	if !HasStagedChanges(dir) {
		return nil
	}
	if err := runCmd(dir, []string{"git", "add", "-A"}, false); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := runCmd(dir, []string{"git", "commit", "-m", commitMsg}, false); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// HasUnpushedCommits reports whether the current branch has commits that are not
// present on baseRef (e.g. "origin/main"). This lets review/ship recognize
// already-committed work that still needs pushing even when the working tree is
// clean. Returns false if the comparison can't be made (e.g. baseRef missing).
func HasUnpushedCommits(dir, baseRef string) bool {
	out, err := runCmdOutput(dir, []string{"git", "rev-list", "--count", baseRef + "..HEAD"})
	if err != nil {
		return false
	}
	n := strings.TrimSpace(out)
	return n != "" && n != "0"
}

// LatestCommitMessage returns the full message (subject + body) of HEAD's most
// recent commit, trimmed. Used to derive a PR title/body for an already-committed
// branch where there's no working-tree diff to summarize.
func LatestCommitMessage(dir string) (string, error) {
	out, err := runCmdOutput(dir, []string{"git", "log", "-1", "--pretty=%B"})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// GitPush commits any working-tree changes (no-op if already committed) and
// pushes to a feature branch using --no-verify to bypass our own pre-push hook.
func GitPush(dir, commitMsg, branch string) error {
	if err := commitAll(dir, commitMsg); err != nil {
		return err
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

// CreatePR creates a GitHub PR from a commit-message-style string. The first
// line of commitMsg becomes the PR title; the remainder (if any) becomes the
// PR body. Splitting is required because GitHub's API rejects titles that
// contain newlines or exceed ~256 chars, so passing the full multi-line `-m`
// payload as the title returns a 422 and surfaces as `gh pr create: exit 1`.
// Returns the PR URL.
func CreatePR(dir, commitMsg, baseBranch string) (string, error) {
	title, body := SplitCommitMessage(commitMsg)
	out, err := runCmdOutput(dir, []string{
		"gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--base", baseBranch,
	})
	if err != nil {
		return "", fmt.Errorf("gh pr create: %w\n%s", err, strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

// SplitCommitMessage splits a commit-style string into (title, body) using
// the first newline as the separator. Surrounding whitespace is trimmed from
// both halves. The body preserves its internal blank lines.
func SplitCommitMessage(msg string) (title, body string) {
	parts := strings.SplitN(strings.TrimSpace(msg), "\n", 2)
	title = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		body = strings.TrimSpace(parts[1])
	}
	return
}

// MergePR merges the given PR using admin privileges. Captures stderr so
// transient failures (mergeability race, branch-protection edge cases)
// surface a real error message instead of an opaque exit code.
func MergePR(dir, prURL string) error {
	out, err := runCmdOutput(dir, []string{
		"gh", "pr", "merge", prURL,
		"--squash", "--admin", "-d",
	})
	if err != nil {
		return fmt.Errorf("gh pr merge: %w\n%s", err, strings.TrimSpace(out))
	}
	return nil
}

// WatchPRChecks waits for the PR checks to complete using gh pr checks --watch.
// It blocks until the pipeline finishes or the timeout expires.
// Returns the run conclusion and any failure logs.
func WatchPRChecks(dir, prURL, branch string, timeout time.Duration) (runID, conclusion string, failureLogs []string, err error) {
	// Wait a few seconds for GitHub to register the push and start checks
	time.Sleep(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "pr", "checks", prURL, "--watch", "--fail-fast")
	cmd.Dir = dir

	// We don't want stdout to pollute our deterministic output, so we run silently
	err = cmd.Run()

	// If err is nil, checks passed successfully
	if err == nil {
		return "", "success", nil, nil
	}

	if ctx.Err() == context.DeadlineExceeded {
		return "", "timeout", nil, fmt.Errorf("CI pipeline did not complete within %s", timeout)
	}

	// Since it failed, we want to grab the exact run ID and failure logs from the CI workflow
	out, listErr := runCmdOutput(dir, []string{
		"gh", "run", "list",
		"--branch", branch,
		"--event", "pull_request",
		"-L", "10",
		"--json", "databaseId,status,conclusion,workflowName",
	})

	if listErr == nil {
		var runs []struct {
			DatabaseID   int64  `json:"databaseId"`
			Status       string `json:"status"`
			Conclusion   string `json:"conclusion"`
			WorkflowName string `json:"workflowName"`
		}
		if jsonErr := json.Unmarshal([]byte(out), &runs); jsonErr == nil {
			for _, run := range runs {
				if run.Conclusion == "failure" && run.WorkflowName == "CI" {
					runID = fmt.Sprintf("%d", run.DatabaseID)
					conclusion = run.Conclusion

					// Fetch failure logs
					logOut, logErr := runCmdOutput(dir, []string{
						"gh", "run", "view", runID, "--log-failed",
					})
					if logErr == nil {
						failureLogs = condenseFailureLogs(logOut)
					}
					break
				}
			}
		}
	}

	if conclusion == "" {
		conclusion = "failure"
	}

	return runID, conclusion, failureLogs, nil
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

// currentBranch returns the current git branch name, or "unknown" on error.
func currentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// ── helpers ──────────────────────────────────────────────────────────────────

func runCmd(dir string, args []string, verbose bool) error {
	if telemetry.IsGoTestCmd(args) {
		var outWriter, errWriter io.Writer
		if verbose {
			outWriter = os.Stdout
			errWriter = os.Stderr
		}
		_, err := telemetry.RunGoTestWithTelemetry(args, dir, outWriter, errWriter, verbose, nil)
		return err
	}

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
