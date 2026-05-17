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

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/mcpinstall"
)

var (
	mcpInstallGlobal bool
	mcpInstallAll    bool
)

var mcpInstallCmd = &cobra.Command{
	Use:   "install [host...]",
	Short: "Install the devx MCP server into one or more agent hosts",
	Long: `Configures supported AI coding agents (Claude Code, Cursor, Codex CLI,
Gemini CLI, Zed, Continue.dev, Windsurf, OpenCode) to launch devx as their
Model Context Protocol server.

Run with no args to auto-detect every installed host and configure each one.
Pass specific host keys (devx mcp list to see them) to target a subset.

By default, hosts that support project-scoped MCP config (Claude Code, Cursor,
OpenCode) are configured at the project level so each repo's tool surface is
sandboxed and travels with the codebase. Use --global to force per-user config.

Examples:
  devx mcp install                    # auto-detect, install everywhere
  devx mcp install claude-code        # target one host
  devx mcp install --all              # install for every supported host (even undetected)
  devx mcp install --global           # use per-user config for hosts that support both
  devx mcp install --dry-run          # show what would change without writing
  devx mcp install --json             # machine-readable output`,
	RunE: runMCPInstall,
}

func init() {
	mcpInstallCmd.Flags().BoolVar(&mcpInstallGlobal, "global", false,
		"Use per-user config for hosts that also support project-scoped config")
	mcpInstallCmd.Flags().BoolVar(&mcpInstallAll, "all", false,
		"Install for every supported host even if not detected on this machine")
	mcpCmd.AddCommand(mcpInstallCmd)
}

func runMCPInstall(_ *cobra.Command, args []string) error {
	opts := mcpinstall.Options{DryRun: DryRun}
	if mcpInstallGlobal {
		opts.Scope = mcpinstall.ScopeUser
	}

	var targets []mcpinstall.Host
	var err error
	switch {
	case mcpInstallAll && len(args) == 0:
		targets = mcpinstall.Hosts()
	default:
		targets, err = mcpinstall.ResolveTargets(args)
		if err != nil {
			return err
		}
	}

	if len(targets) == 0 {
		if outputJSON {
			return json.NewEncoder(os.Stdout).Encode([]mcpinstall.InstallResult{})
		}
		fmt.Println("🔍 No supported agent tools detected on this machine.")
		fmt.Println("   Run 'devx mcp list' to see what devx can integrate with.")
		return nil
	}

	results := make([]mcpinstall.InstallResult, 0, len(targets))
	for _, h := range targets {
		res, _ := h.Install(opts)
		results = append(results, res)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(results)
	}

	fmt.Println("📝 devx MCP install")
	if DryRun {
		fmt.Println("   (dry-run — no files were modified)")
	}
	for _, r := range results {
		marker := "✓"
		if !r.OK {
			marker = "✗"
		} else if !r.Changed {
			marker = "•"
		}
		fmt.Printf("   %s %-14s → %s  (%s)\n",
			marker, r.HostKey, displayPath(r.ConfigPath, r.Scope), r.Message)
	}
	fmt.Println()
	fmt.Println("  Restart your agent (or open a new chat) to load devx tools.")
	return nil
}

// displayPath formats config-path output for readability. Manual-only hosts
// have no path, so we display "(manual — see message)" instead.
func displayPath(path, scope string) string {
	if path == "" {
		return "(manual — see message)"
	}
	if scope != "" {
		return fmt.Sprintf("%s [%s]", path, scope)
	}
	return path
}
