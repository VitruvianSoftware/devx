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

// Package ship — hook.go provides the pre-push hook content and installation.
package ship

import (
	"fmt"
	"os"
	"path/filepath"
)

// PreCommitHookContent is the shell script installed into .git/hooks/pre-commit.
// It prevents direct commits to protected trunk branches and forces developers
// to work on feature branches.
// Humans can bypass with: git commit --no-verify
const PreCommitHookContent = `#!/bin/sh
# ═══════════════════════════════════════════════════════════════════════════
# devx pre-commit hook — Protected Branch Guardrail
# ═══════════════════════════════════════════════════════════════════════════
# This hook is installed by 'devx audit install-hooks' or
# 'devx agent ship --install-hook'.
#
# It prevents accidental commits to protected trunk branches
# (main, master, develop, development, dev), ensuring developers
# always work on feature branches.
#
# Bypass (if you know what you're doing): git commit --no-verify
# ═══════════════════════════════════════════════════════════════════════════

BRANCH="$(git rev-parse --abbrev-ref HEAD)"

case "$BRANCH" in
  main|master|develop|development|dev)
    echo ""
    echo "╭──────────────────────────────────────────────────────────────────╮"
    echo "│  ❌ Direct commits to '${BRANCH}' are blocked by devx.           │"
    echo "│                                                                  │"
    echo "│  Please create a feature branch first:                           │"
    echo "│    git checkout -b <your-feature-branch>                         │"
    echo "│                                                                  │"
    echo "│  Bypass (not recommended): git commit --no-verify                │"
    echo "╰──────────────────────────────────────────────────────────────────╯"
    echo ""
    exit 1
    ;;
esac

exit 0
`

// PrePushHookContent is the shell script installed into .git/hooks/pre-push.
// It runs devx audit (secrets + vulnerability scanning) first, then blocks
// all git push attempts and directs agents to use devx agent ship.
// Humans can bypass with: git push --no-verify
const PrePushHookContent = `#!/bin/sh
# ═══════════════════════════════════════════════════════════════════════════
# devx pre-push hook — Security Audit + Agentic Pipeline Guardrail
# ═══════════════════════════════════════════════════════════════════════════
# This hook is installed by 'devx agent ship --install-hook'.
# It performs two functions:
#   1. Runs devx audit (secrets + vulnerability scanning)
#   2. Blocks direct 'git push' and steers AI agents to 'devx agent review'
#      (open a PR for human review) or 'devx agent ship' (merge after CI)
#
# Humans can bypass this hook at any time with: git push --no-verify
# ═══════════════════════════════════════════════════════════════════════════

# Step 1: Security audit
echo "🔍 devx audit: scanning for secrets and vulnerabilities..."
devx audit
if [ $? -ne 0 ]; then
  echo "❌ devx audit failed — fix issues before pushing."
  exit 1
fi

# Step 2: Block direct push (agents must use devx agent review/ship)
echo ""
echo "╭──────────────────────────────────────────────────────────────────╮"
echo "│  ✋ Direct 'git push' is blocked by devx.                         │"
echo "│                                                                  │"
echo "│  AI agents, choose by intent:                                    │"
echo "│    Open a PR for review:  devx agent review -m \"msg\"             │"
echo "│    Merge after CI passes: devx agent ship   -m \"msg\"             │"
echo "│  Humans can bypass:       git push --no-verify                   │"
echo "│                                                                  │"
echo "│  This guardrail ensures pre-flight checks and CI verification    │"
echo "│  are never skipped.                                              │"
echo "╰──────────────────────────────────────────────────────────────────╯"
echo ""
exit 1
`

// InstallPreCommitHook writes the pre-commit hook into .git/hooks/.
// If any devx-managed hook already exists, it will be safely overwritten.
// If a non-devx hook exists, it returns an error to avoid clobbering.
func InstallPreCommitHook(repoDir string) error {
	hookDir := filepath.Join(repoDir, ".git", "hooks")
	hookPath := filepath.Join(hookDir, "pre-commit")

	// Ensure hooks directory exists
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks directory: %w", err)
	}

	// Check for existing non-devx hook
	if data, err := os.ReadFile(hookPath); err == nil {
		content := string(data)
		if content != "" && !isDevxHook(content) {
			return fmt.Errorf("existing non-devx pre-commit hook found at %s — refusing to overwrite. Remove it manually or merge the hooks", hookPath)
		}
	}

	if err := os.WriteFile(hookPath, []byte(PreCommitHookContent), 0o755); err != nil {
		return fmt.Errorf("writing pre-commit hook: %w", err)
	}

	return nil
}

// InstallPrePushHook writes the pre-push hook into .git/hooks/.
// If any devx-managed hook already exists, it will be safely overwritten.
// If a non-devx hook exists, it returns an error to avoid clobbering.
func InstallPrePushHook(repoDir string) error {
	hookDir := filepath.Join(repoDir, ".git", "hooks")
	hookPath := filepath.Join(hookDir, "pre-push")

	// Ensure hooks directory exists
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks directory: %w", err)
	}

	// Check for existing non-devx hook
	if data, err := os.ReadFile(hookPath); err == nil {
		content := string(data)
		if content != "" && !isDevxHook(content) {
			return fmt.Errorf("existing non-devx pre-push hook found at %s — refusing to overwrite. Remove it manually or merge the hooks", hookPath)
		}
	}

	if err := os.WriteFile(hookPath, []byte(PrePushHookContent), 0o755); err != nil {
		return fmt.Errorf("writing pre-push hook: %w", err)
	}

	return nil
}

// IsPreCommitHookInstalled checks if a devx pre-commit hook is present.
func IsPreCommitHookInstalled(repoDir string) bool {
	hookPath := filepath.Join(repoDir, ".git", "hooks", "pre-commit")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}
	return isDevxHook(string(data))
}

// IsPrePushHookInstalled checks if a devx pre-push hook is present.
func IsPrePushHookInstalled(repoDir string) bool {
	hookPath := filepath.Join(repoDir, ".git", "hooks", "pre-push")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}
	return isDevxHook(string(data))
}

func isDevxHook(content string) bool {
	return len(content) > 0 && containsAny(content,
		"devx agent ship",
		"devx pre-push hook",
		"devx pre-commit hook",
		"Agentic Pipeline Guardrail",
		"Protected Branch Guardrail",
		"devx audit",
		"Installed by devx",
	)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) > 0 && len(sub) > 0 && indexOf(s, sub) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
