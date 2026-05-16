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
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// hookCmd is a hidden command group invoked by git hooks.
// Users should never call this directly.
var hookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Internal commands invoked by git hooks",
	Hidden: true,
}

var hookPreCommitCmd = &cobra.Command{
	Use:   "pre-commit",
	Short: "Invoked by the .git/hooks/pre-commit hook",
	RunE: func(_ *cobra.Command, _ []string) error {
		// Resolve current branch
		branch := "unknown"
		if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
			branch = strings.TrimSpace(string(out))
		}

		protected := map[string]bool{
			"main": true, "master": true,
			"develop": true, "development": true, "dev": true,
		}

		if protected[branch] {
			_, _ = fmt.Fprintln(os.Stderr)
			_, _ = fmt.Fprintln(os.Stderr, "╭──────────────────────────────────────────────────────────────────╮")
			_, _ = fmt.Fprintf(os.Stderr, "│  ❌ Direct commits to '%s' are blocked by devx.%s│\n", branch, strings.Repeat(" ", 15-len(branch)))
			_, _ = fmt.Fprintln(os.Stderr, "│                                                                  │")
			_, _ = fmt.Fprintln(os.Stderr, "│  Please create a feature branch first:                           │")
			_, _ = fmt.Fprintln(os.Stderr, "│    git checkout -b <your-feature-branch>                         │")
			_, _ = fmt.Fprintln(os.Stderr, "│                                                                  │")
			_, _ = fmt.Fprintln(os.Stderr, "│  Bypass (not recommended): git commit --no-verify                │")
			_, _ = fmt.Fprintln(os.Stderr, "╰──────────────────────────────────────────────────────────────────╯")
			_, _ = fmt.Fprintln(os.Stderr)
			os.Exit(1)
		}
		return nil
	},
}

var hookPrePushCmd = &cobra.Command{
	Use:   "pre-push",
	Short: "Invoked by the .git/hooks/pre-push hook",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintln(os.Stderr)
		_, _ = fmt.Fprintln(os.Stderr, "╭──────────────────────────────────────────────────────────────────╮")
		_, _ = fmt.Fprintln(os.Stderr, "│  ✋ Direct 'git push' is blocked by devx.                        │")
		_, _ = fmt.Fprintln(os.Stderr, "│                                                                  │")
		_, _ = fmt.Fprintln(os.Stderr, "│  AI Agents MUST use:   devx agent ship -m \"commit message\"       │")
		_, _ = fmt.Fprintln(os.Stderr, "│  Humans can bypass:    git push --no-verify                      │")
		_, _ = fmt.Fprintln(os.Stderr, "│                                                                  │")
		_, _ = fmt.Fprintln(os.Stderr, "│  This guardrail ensures pre-flight checks and CI verification    │")
		_, _ = fmt.Fprintln(os.Stderr, "│  are never skipped.                                              │")
		_, _ = fmt.Fprintln(os.Stderr, "╰──────────────────────────────────────────────────────────────────╯")
		_, _ = fmt.Fprintln(os.Stderr)
		os.Exit(1)
		return nil
	},
}

func init() {
	hookCmd.AddCommand(hookPreCommitCmd)
	hookCmd.AddCommand(hookPrePushCmd)
	rootCmd.AddCommand(hookCmd)
}
