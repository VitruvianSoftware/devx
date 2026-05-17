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

var mcpUninstallGlobal bool

var mcpUninstallCmd = &cobra.Command{
	Use:   "uninstall [host...]",
	Short: "Remove the devx MCP server from one or more agent hosts",
	Long: `Removes the devx entry from each targeted host's MCP config. Leaves any
other MCP servers (and the host's own settings) untouched.

Run with no args to remove from every detected host. Pass specific host keys
to target a subset. Hosts that use plugin/marketplace models (Cowork,
Antigravity) require manual removal — devx will print instructions.

Examples:
  devx mcp uninstall              # remove from every detected host
  devx mcp uninstall codex zed    # target specific hosts
  devx mcp uninstall --dry-run    # show what would change`,
	RunE: runMCPUninstall,
}

func init() {
	mcpUninstallCmd.Flags().BoolVar(&mcpUninstallGlobal, "global", false,
		"Use per-user config for hosts that also support project-scoped config")
	mcpCmd.AddCommand(mcpUninstallCmd)
}

func runMCPUninstall(_ *cobra.Command, args []string) error {
	opts := mcpinstall.Options{DryRun: DryRun}
	if mcpUninstallGlobal {
		opts.Scope = mcpinstall.ScopeUser
	}

	targets, err := mcpinstall.ResolveTargets(args)
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		if outputJSON {
			return json.NewEncoder(os.Stdout).Encode([]mcpinstall.InstallResult{})
		}
		fmt.Println("🔍 No supported agent tools detected on this machine.")
		return nil
	}

	results := make([]mcpinstall.InstallResult, 0, len(targets))
	for _, h := range targets {
		res, _ := h.Uninstall(opts)
		results = append(results, res)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(results)
	}

	fmt.Println("🧹 devx MCP uninstall")
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
	return nil
}
