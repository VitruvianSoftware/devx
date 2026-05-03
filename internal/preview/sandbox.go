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

// Package preview provides orchestration logic for the 'devx preview' command.
// It manages isolated PR sandbox environments using git worktrees, namespaced
// database containers, and per-sandbox tunnel configurations.
package preview

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/cloudflare"
	"github.com/VitruvianSoftware/devx/internal/exposure"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

// Sandbox holds all state for an isolated PR preview environment.
type Sandbox struct {
	PRNumber     int
	BranchName   string
	WorktreeDir  string
	ProjectName  string // namespace prefix for containers (e.g., "pr-42")
	TunnelName   string // derived from project name
	LocalBranch  string // local tracking branch (e.g., "devx-pr-42")
	errors       []string
}

// New creates a Sandbox with all derived names computed from the PR number.
func New(prNumber int) *Sandbox {
	return &Sandbox{
		PRNumber:    prNumber,
		ProjectName: fmt.Sprintf("pr-%d", prNumber),
		LocalBranch: fmt.Sprintf("devx-pr-%d", prNumber),
		WorktreeDir: filepath.Join(os.TempDir(), fmt.Sprintf("devx-preview-%d", prNumber)),
	}
}

// Setup validates prerequisites, fetches the PR, and creates a git worktree.
func (s *Sandbox) Setup() error {
	// 1. Validate gh CLI is installed
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("GitHub CLI is required for devx preview. Install: brew install gh && gh auth login")
	}

	// 2. Validate gh CLI is authenticated
	if err := exec.Command("gh", "auth", "status").Run(); err != nil {
		return fmt.Errorf("GitHub CLI is not authenticated. Run: gh auth login")
	}

	// 3. Fetch the PR branch name
	out, err := exec.Command("gh", "pr", "view", fmt.Sprintf("%d", s.PRNumber),
		"--json", "headRefName", "--jq", ".headRefName").Output()
	if err != nil {
		return fmt.Errorf("PR #%d not found in this repository — verify the PR number and your gh auth", s.PRNumber)
	}
	s.BranchName = strings.TrimSpace(string(out))
	if s.BranchName == "" {
		return fmt.Errorf("could not determine branch name for PR #%d", s.PRNumber)
	}

	fmt.Printf("%s Fetching PR #%d (branch: %s)...\n", tui.IconRunning, s.PRNumber, s.BranchName)

	// 4. Fetch the PR ref
	fetchCmd := exec.Command("git", "fetch", "origin",
		fmt.Sprintf("pull/%d/head:%s", s.PRNumber, s.LocalBranch))
	fetchCmd.Stdout = os.Stdout
	fetchCmd.Stderr = os.Stderr
	if err := fetchCmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch PR #%d: %w (fork PRs are not supported in this version)", s.PRNumber, err)
	}

	// 5. Create the worktree
	fmt.Printf("%s Creating isolated worktree at %s...\n", tui.IconRunning, s.WorktreeDir)
	wtCmd := exec.Command("git", "worktree", "add", s.WorktreeDir, s.LocalBranch)
	wtCmd.Stdout = os.Stdout
	wtCmd.Stderr = os.Stderr
	if err := wtCmd.Run(); err != nil {
		return fmt.Errorf("failed to create git worktree: %w", err)
	}

	// 6. Verify devx.yaml exists in the worktree
	if _, err := os.Stat(filepath.Join(s.WorktreeDir, "devx.yaml")); os.IsNotExist(err) {
		fmt.Printf("⚠️  No devx.yaml found in PR #%d — services will not be started, but the worktree is available at %s\n", s.PRNumber, s.WorktreeDir)
	}

	return nil
}

// Run starts `devx up` inside the worktree with the project override.
// It blocks until the subprocess exits or the context is cancelled.
func (s *Sandbox) Run(ctx context.Context, globalFlags []string) error {
	devxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving devx binary: %w", err)
	}

	args := []string{"up"}
	args = append(args, globalFlags...)

	cmd := exec.CommandContext(ctx, devxBin, args...)
	cmd.Dir = s.WorktreeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Inject the project override so devx up namespaces databases
	cmd.Env = append(os.Environ(), fmt.Sprintf("DEVX_PROJECT_OVERRIDE=%s", s.ProjectName))

	fmt.Printf("\n%s Starting sandbox for PR #%d (project: %s)...\n\n", tui.IconRunning, s.PRNumber, s.ProjectName)

	if err := cmd.Run(); err != nil {
		// Context cancellation (Ctrl+C) is expected — not an error
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("devx up exited with error: %w", err)
	}
	return nil
}

// Teardown performs best-effort cleanup of all sandbox resources.
// Errors are collected rather than fatal — we want to clean up as much as possible.
func (s *Sandbox) Teardown() {
	fmt.Printf("\n🧹 Cleaning up sandbox for PR #%d...\n", s.PRNumber)

	// 1. Remove namespaced database containers via devx db rm
	s.teardownDatabases()

	// 2. Clean up Cloudflare tunnel if one was created
	s.teardownTunnel()

	// 3. Remove the git worktree
	s.teardownWorktree()

	// 4. Delete the temporary local branch
	s.teardownBranch()

	// Report results
	if len(s.errors) > 0 {
		fmt.Printf("\n⚠️  Some cleanup steps had issues:\n")
		for _, e := range s.errors {
			fmt.Printf("   • %s\n", e)
		}
	} else {
		fmt.Printf("%s Sandbox for PR #%d cleaned up successfully.\n", tui.IconDone, s.PRNumber)
	}
}

func (s *Sandbox) teardownDatabases() {
	devxBin, err := os.Executable()
	if err != nil {
		s.errors = append(s.errors, fmt.Sprintf("could not resolve devx binary: %v", err))
		return
	}

	// Discover database engines from worktree's devx.yaml
	engines := s.discoverEngines()
	for _, engine := range engines {
		fmt.Printf("  %s Removing database: %s (project: %s)\n", tui.IconRunning, engine, s.ProjectName)
		cmd := exec.Command(devxBin, "db", "rm", engine, "--project", s.ProjectName, "-y")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			s.errors = append(s.errors, fmt.Sprintf("failed removing %s database: %v", engine, err))
		}
	}
}

func (s *Sandbox) teardownTunnel() {
	// Look for tunnels with our project name prefix
	devName := os.Getenv("USER")
	prefix := fmt.Sprintf("devx-expose-%s-prj-%s-", devName, s.ProjectName)

	tunnels, err := cloudflare.ListExposedTunnels(devName)
	if err != nil {
		s.errors = append(s.errors, fmt.Sprintf("could not list tunnels: %v", err))
		return
	}

	for _, t := range tunnels {
		if strings.HasPrefix(t.Name, prefix) {
			fmt.Printf("  %s Removing tunnel: %s\n", tui.IconRunning, t.Name)
			out, delErr := exec.Command("cloudflared", "tunnel", "delete", "-f", t.Name).CombinedOutput()
			if delErr != nil {
				s.errors = append(s.errors, fmt.Sprintf("failed deleting tunnel %s: %v — %s", t.Name, delErr, string(out)))
			}
			// Clean up exposure store entry
			_ = exposure.RemoveByName(t.Name)
		}
	}
}

func (s *Sandbox) teardownWorktree() {
	if s.WorktreeDir == "" {
		return
	}
	fmt.Printf("  %s Removing worktree: %s\n", tui.IconRunning, s.WorktreeDir)
	cmd := exec.Command("git", "worktree", "remove", "--force", s.WorktreeDir)
	if err := cmd.Run(); err != nil {
		// Fallback: remove directory manually
		if rmErr := os.RemoveAll(s.WorktreeDir); rmErr != nil {
			s.errors = append(s.errors, fmt.Sprintf("failed removing worktree: %v (manual removal also failed: %v)", err, rmErr))
			return
		}
		// Prune stale worktree references
		_ = exec.Command("git", "worktree", "prune").Run()
	}
}

func (s *Sandbox) teardownBranch() {
	fmt.Printf("  %s Deleting local branch: %s\n", tui.IconRunning, s.LocalBranch)
	if err := exec.Command("git", "branch", "-D", s.LocalBranch).Run(); err != nil {
		s.errors = append(s.errors, fmt.Sprintf("failed deleting branch %s: %v", s.LocalBranch, err))
	}
}

// discoverEngines reads the worktree's devx.yaml to find database engine names.
// Falls back to scanning for containers with the project prefix if YAML parsing fails.
func (s *Sandbox) discoverEngines() []string {
	// Simple approach: scan for containers matching our project prefix
	// This works even if devx.yaml parsing would require importing the config package
	out, err := exec.Command("podman", "ps", "-a",
		"--filter", fmt.Sprintf("name=devx-db-%s-", s.ProjectName),
		"--format", "{{.Names}}").Output()
	if err != nil {
		// Try docker as fallback
		out, err = exec.Command("docker", "ps", "-a",
			"--filter", fmt.Sprintf("name=devx-db-%s-", s.ProjectName),
			"--format", "{{.Names}}").Output()
		if err != nil {
			return nil
		}
	}

	var engines []string
	prefix := fmt.Sprintf("devx-db-%s-", s.ProjectName)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		engine := strings.TrimPrefix(line, prefix)
		if engine != line { // prefix was actually present
			engines = append(engines, engine)
		}
	}
	return engines
}

// DryRun returns a human-readable summary of what would happen.
func (s *Sandbox) DryRun() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "DRY RUN: devx preview %d\n\n", s.PRNumber)
	fmt.Fprintf(&sb, "  PR Number:      %d\n", s.PRNumber)
	fmt.Fprintf(&sb, "  Project Name:   %s\n", s.ProjectName)
	fmt.Fprintf(&sb, "  Local Branch:   %s\n", s.LocalBranch)
	fmt.Fprintf(&sb, "  Worktree Path:  %s\n", s.WorktreeDir)
	fmt.Fprintf(&sb, "  DB Prefix:      devx-db-%s-<engine>\n", s.ProjectName)
	fmt.Fprintf(&sb, "  Volume Prefix:  devx-data-%s-<engine>\n", s.ProjectName)
	sb.WriteString("\n  Actions:\n")
	fmt.Fprintf(&sb, "    1. gh pr view %d --json headRefName\n", s.PRNumber)
	fmt.Fprintf(&sb, "    2. git fetch origin pull/%d/head:%s\n", s.PRNumber, s.LocalBranch)
	fmt.Fprintf(&sb, "    3. git worktree add %s %s\n", s.WorktreeDir, s.LocalBranch)
	fmt.Fprintf(&sb, "    4. DEVX_PROJECT_OVERRIDE=%s devx up\n", s.ProjectName)
	sb.WriteString("\n  On exit (Ctrl+C):\n")
	fmt.Fprintf(&sb, "    5. devx db rm <engine> --project %s -y\n", s.ProjectName)
	sb.WriteString("    6. cloudflared tunnel delete -f <tunnel-name>\n")
	fmt.Fprintf(&sb, "    7. git worktree remove --force %s\n", s.WorktreeDir)
	fmt.Fprintf(&sb, "    8. git branch -D %s\n", s.LocalBranch)
	return sb.String()
}

// JSON returns a structured representation for --json output.
func (s *Sandbox) JSON() ([]byte, error) {
	data := map[string]interface{}{
		"pr_number":    s.PRNumber,
		"project_name": s.ProjectName,
		"local_branch": s.LocalBranch,
		"worktree_dir": s.WorktreeDir,
		"db_prefix":    fmt.Sprintf("devx-db-%s", s.ProjectName),
		"volume_prefix": fmt.Sprintf("devx-data-%s", s.ProjectName),
	}
	return json.MarshalIndent(data, "", "  ")
}
