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

package mcpinstall

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// manualHost is used for agent hosts whose config format/location isn't
// stable enough to auto-write, or where the integration model is via a
// plugin/marketplace rather than a flat MCP config file. The adapter detects
// the host and prints a copy-pasteable JSON snippet with instructions
// instead of mutating any file. Safe by construction.
type manualHost struct {
	displayName string
	key         string
	detectBins  []string
	detectPaths []string
	// instructions is shown to the user; describes where to paste the
	// snippet and any host-specific gotchas.
	instructions string
}

func (h *manualHost) Key() string         { return h.key }
func (h *manualHost) DisplayName() string { return h.displayName }

func (h *manualHost) Detect() bool {
	for _, bin := range h.detectBins {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	for _, p := range h.detectPaths {
		if _, err := os.Stat(expandHome(p)); err == nil {
			return true
		}
	}
	return false
}

func (h *manualHost) Status(_ Options) (Status, error) {
	return Status{
		HostKey:     h.key,
		DisplayName: h.displayName,
		Detected:    h.Detect(),
		Scope:       "manual",
		Note:        "manual configuration only — devx cannot auto-detect install state",
	}, nil
}

func (h *manualHost) Install(opts Options) (InstallResult, error) {
	snippet := manualSnippet(opts)
	return InstallResult{
		HostKey:     h.key,
		DisplayName: h.displayName,
		OK:          true,
		Changed:     false,
		Scope:       "manual",
		Message:     h.instructions + "\n\n" + snippet,
	}, nil
}

func (h *manualHost) Uninstall(_ Options) (InstallResult, error) {
	return InstallResult{
		HostKey:     h.key,
		DisplayName: h.displayName,
		OK:          true,
		Scope:       "manual",
		Message:     "remove the \"devx\" entry from " + h.displayName + "'s MCP/plugin configuration manually",
	}, nil
}

// manualSnippet renders the standard MCP server entry as a JSON object so
// users can paste it into whatever format their host expects.
func manualSnippet(opts Options) string {
	entry := devxEntry(opts)
	b, _ := json.MarshalIndent(map[string]any{ServerEntryKey: entry}, "", "  ")
	return fmt.Sprintf("  Server entry (paste under your host's `mcpServers` or equivalent):\n%s", indent(string(b), 4))
}

func indent(s string, n int) string {
	pad := ""
	for i := 0; i < n; i++ {
		pad += " "
	}
	out := pad
	for _, r := range s {
		out += string(r)
		if r == '\n' {
			out += pad
		}
	}
	return out
}
