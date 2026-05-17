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

import "github.com/spf13/cobra"

var mcpCmd = &cobra.Command{
	Use:     "mcp",
	GroupID: "ci",
	Short:   "Expose devx as a Model Context Protocol (MCP) server for AI coding agents",
	Long: `Run devx as a Model Context Protocol server so AI coding agents (Claude Code,
Cursor, Codex CLI, Gemini CLI, Zed, Continue, Windsurf, and more) can call
devx commands as first-class typed tools instead of shelling out and parsing
text.

Subcommands:
  serve       Start the MCP server (stdio transport — invoked by agent hosts)
  install     Install the devx MCP server into one or more detected agent hosts
  uninstall   Remove the devx MCP server from one or more agent hosts
  status      Show where devx is currently installed
  doctor      Validate every install actually works end-to-end
  list        List supported agent hosts (and which are detected on this machine)`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
