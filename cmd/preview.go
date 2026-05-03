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
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/VitruvianSoftware/devx/internal/preview"
	"github.com/spf13/cobra"
)

var previewCmd = &cobra.Command{
	Use:     "preview <pr-number>",
	GroupID: "orchestration",
	Short:   "Spin up an isolated sandbox to review a PR without switching branches",
	Long: `Creates an isolated environment for reviewing a pull request.

Uses git worktrees to check out the PR code without disrupting your current
branch. Databases are provisioned with a unique namespace so they don't collide
with your active development environment. Cloudflare tunnels are created with
PR-specific names for independent public access.

On exit (Ctrl+C), the worktree, databases, tunnels, and temporary branches are
cleaned up automatically.

Prerequisites:
  • GitHub CLI (gh) must be installed and authenticated
  • The current directory must be inside a git repository

Limitations:
  • Fork PRs are not supported in this version
  • One preview at a time is recommended to avoid port conflicts`,
	Example: `  # Preview PR #42
  devx preview 42

  # Dry-run to see what would happen
  devx preview 42 --dry-run

  # Non-interactive mode (auto-confirm all prompts)
  devx preview 42 -y`,
	Args: cobra.ExactArgs(1),
	RunE: runPreview,
}

func init() {
	rootCmd.AddCommand(previewCmd)
}

func runPreview(_ *cobra.Command, args []string) error {
	prNumber, err := strconv.Atoi(args[0])
	if err != nil || prNumber <= 0 {
		return fmt.Errorf("invalid PR number %q — must be a positive integer", args[0])
	}

	sandbox := preview.New(prNumber)

	// --dry-run: show what would happen without executing
	if DryRun {
		fmt.Print(sandbox.DryRun())
		return nil
	}

	// --json: emit structured output
	if outputJSON {
		data, err := sandbox.JSON()
		if err != nil {
			return fmt.Errorf("failed to generate JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Setup: fetch PR, create worktree
	if err := sandbox.Setup(); err != nil {
		return err
	}

	// Register signal handler for automatic cleanup
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\n\n⚡ Signal received — initiating sandbox teardown...")
		cancel()
	}()

	// Build global flags to forward to the subprocess
	var globalFlags []string
	if NonInteractive {
		globalFlags = append(globalFlags, "-y")
	}
	if envFile != ".env" {
		globalFlags = append(globalFlags, "--env-file", envFile)
	}

	// Run devx up inside the worktree
	runErr := sandbox.Run(ctx, globalFlags)

	// Always teardown, regardless of how we exited
	sandbox.Teardown()

	signal.Stop(sigCh)

	return runErr
}
