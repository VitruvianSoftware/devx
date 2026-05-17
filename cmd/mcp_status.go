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

var mcpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show where the devx MCP server is currently installed",
	RunE:  runMCPStatus,
}

func init() {
	mcpCmd.AddCommand(mcpStatusCmd)
}

func runMCPStatus(_ *cobra.Command, _ []string) error {
	hosts := mcpinstall.Hosts()
	results := make([]mcpinstall.Status, 0, len(hosts))
	for _, h := range hosts {
		st, _ := h.Status(mcpinstall.Options{})
		results = append(results, st)
	}

	if outputJSON {
		return json.NewEncoder(os.Stdout).Encode(results)
	}

	fmt.Println("📋 devx MCP — install status")
	for _, st := range results {
		var marker, note string
		switch {
		case st.Installed:
			marker = "✓"
			note = "installed"
		case st.Detected:
			marker = "○"
			note = "detected but not installed"
		default:
			marker = "·"
			note = "not detected"
		}
		if st.Note != "" {
			note = note + " — " + st.Note
		}
		fmt.Printf("   %s %-14s  %s\n", marker, st.HostKey, note)
		if st.Installed && st.ConfigPath != "" {
			fmt.Printf("       %s\n", st.ConfigPath)
		}
	}
	_, _ = fmt.Fprintln(os.Stderr) // separator before any future warnings
	return nil
}
