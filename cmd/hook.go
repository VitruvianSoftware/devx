package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// hookCmd is a hidden command group invoked by git hooks.
// Users should never call this directly.
var hookCmd = &cobra.Command{
	Use:    "hook",
	Short:  "Internal commands invoked by git hooks",
	Hidden: true,
}

var hookPrePushCmd = &cobra.Command{
	Use:   "pre-push",
	Short: "Invoked by the .git/hooks/pre-push hook",
	RunE: func(_ *cobra.Command, _ []string) error {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "╭──────────────────────────────────────────────────────────────────╮")
		fmt.Fprintln(os.Stderr, "│  ✋ Direct 'git push' is blocked by devx.                        │")
		fmt.Fprintln(os.Stderr, "│                                                                  │")
		fmt.Fprintln(os.Stderr, "│  AI Agents MUST use:   devx agent ship -m \"commit message\"       │")
		fmt.Fprintln(os.Stderr, "│  Humans can bypass:    git push --no-verify                      │")
		fmt.Fprintln(os.Stderr, "│                                                                  │")
		fmt.Fprintln(os.Stderr, "│  This guardrail ensures pre-flight checks and CI verification    │")
		fmt.Fprintln(os.Stderr, "│  are never skipped.                                              │")
		fmt.Fprintln(os.Stderr, "╰──────────────────────────────────────────────────────────────────╯")
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
		return nil
	},
}

func init() {
	hookCmd.AddCommand(hookPrePushCmd)
	rootCmd.AddCommand(hookCmd)
}
