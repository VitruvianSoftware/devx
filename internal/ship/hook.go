// Package ship — hook.go provides the pre-push hook content and installation.
package ship

import (
	"fmt"
	"os"
	"path/filepath"
)

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
#   2. Blocks direct 'git push' and forces AI agents to use 'devx agent ship'
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

# Step 2: Block direct push (agents must use devx agent ship)
echo ""
echo "╭──────────────────────────────────────────────────────────────────╮"
echo "│  ✋ Direct 'git push' is blocked by devx.                        │"
echo "│                                                                  │"
echo "│  AI Agents MUST use:   devx agent ship -m \"commit message\"       │"
echo "│  Humans can bypass:    git push --no-verify                      │"
echo "│                                                                  │"
echo "│  This guardrail ensures pre-flight checks and CI verification    │"
echo "│  are never skipped.                                              │"
echo "╰──────────────────────────────────────────────────────────────────╯"
echo ""
exit 1
`

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
		"Agentic Pipeline Guardrail",
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
