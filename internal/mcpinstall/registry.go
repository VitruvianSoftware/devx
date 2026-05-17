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
	"fmt"
	"sort"
)

// Hosts returns the registered set of agent-host adapters. Adding support for
// a new host means adding an entry here — every other place in the package is
// host-agnostic.
//
// Config paths and detection heuristics are best-effort and current as of the
// 2026 ecosystem. Hosts whose config format/location is unstable or whose
// integration model is plugin-based use the manualHost adapter and emit a
// copy-pasteable snippet instead.
func Hosts() []Host {
	return []Host{
		// Claude Code — Anthropic's official CLI. Project-scoped .mcp.json
		// is the recommended location and travels with the codebase.
		&jsonHost{
			displayName:   "Claude Code",
			key:           "claude-code",
			detectBins:    []string{"claude"},
			detectPaths:   []string{"~/.claude", "~/.claude.json"},
			projectConfig: ".mcp.json",
			userConfig:    "~/.claude.json",
		},

		// Cursor — VSCode-based AI editor. Project-scoped lives under
		// .cursor/mcp.json; global at ~/.cursor/mcp.json.
		&jsonHost{
			displayName:   "Cursor",
			key:           "cursor",
			detectBins:    []string{"cursor"},
			detectPaths:   []string{"~/Applications/Cursor.app", "/Applications/Cursor.app", "~/.cursor"},
			projectConfig: ".cursor/mcp.json",
			userConfig:    "~/.cursor/mcp.json",
		},

		// Codex CLI — OpenAI's terminal coding agent. TOML config.
		codexHost{},

		// Gemini CLI — Google's open-source CLI. JSON config under settings.
		&jsonHost{
			displayName: "Gemini CLI",
			key:         "gemini-cli",
			detectBins:  []string{"gemini"},
			detectPaths: []string{"~/.gemini"},
			userConfig:  "~/.gemini/settings.json",
		},

		// Zed — Rust-based editor. Uses `context_servers` key.
		&jsonHost{
			displayName:  "Zed",
			key:          "zed",
			detectBins:   []string{"zed"},
			detectPaths:  []string{"~/.config/zed", "/Applications/Zed.app", "~/Applications/Zed.app"},
			userConfig:   "~/.config/zed/settings.json",
			containerKey: "context_servers",
		},

		// Continue.dev — VSCode/JetBrains plugin. JSON config.
		&jsonHost{
			displayName: "Continue.dev",
			key:         "continue",
			detectPaths: []string{"~/.continue"},
			userConfig:  "~/.continue/config.json",
		},

		// Windsurf (Codeium) — JSON config.
		&jsonHost{
			displayName: "Windsurf",
			key:         "windsurf",
			detectBins:  []string{"windsurf"},
			detectPaths: []string{"~/.codeium/windsurf", "/Applications/Windsurf.app"},
			userConfig:  "~/.codeium/windsurf/mcp_config.json",
		},

		// OpenCode (sst/opencode and similar). Conventional config path
		// follows XDG. Detection is best-effort; manual snippet on miss.
		&jsonHost{
			displayName:   "OpenCode",
			key:           "opencode",
			detectBins:    []string{"opencode"},
			detectPaths:   []string{"~/.config/opencode", "~/.opencode"},
			projectConfig: ".opencode/mcp.json",
			userConfig:    "~/.config/opencode/mcp.json",
		},

		// Cowork — Anthropic's workplace-focused desktop product. Uses a
		// plugin/marketplace model rather than a flat MCP config file, so
		// we emit instructions instead of mutating any file.
		&manualHost{
			displayName: "Cowork",
			key:         "cowork",
			detectPaths: []string{"~/Library/Application Support/Cowork", "/Applications/Cowork.app"},
			instructions: "Cowork integrates MCP via its plugin/connector system. " +
				"In Cowork: Settings → Connectors → Add MCP Server. " +
				"Use the JSON snippet below as the server definition.",
		},

		// Antigravity — Google's IDE (late 2025). Config layout still
		// stabilizing; emit instructions.
		&manualHost{
			displayName: "Antigravity",
			key:         "antigravity",
			detectBins:  []string{"antigravity"},
			detectPaths: []string{"/Applications/Antigravity.app", "~/.antigravity"},
			instructions: "Antigravity supports MCP servers via its settings UI. " +
				"Open Settings → AI → Model Context Protocol → Add Server, then paste the snippet below.",
		},
	}
}

// HostByKey looks up a host by its stable key. Returns nil if not registered.
func HostByKey(key string) Host {
	for _, h := range Hosts() {
		if h.Key() == key {
			return h
		}
	}
	return nil
}

// AllHostKeys returns the registered host keys in alphabetical order.
func AllHostKeys() []string {
	hs := Hosts()
	keys := make([]string, 0, len(hs))
	for _, h := range hs {
		keys = append(keys, h.Key())
	}
	sort.Strings(keys)
	return keys
}

// ResolveTargets parses a list of host keys, returning matched hosts. If
// names is empty, returns every detected host. An unknown key produces an
// error with a hint of supported names.
func ResolveTargets(names []string) ([]Host, error) {
	if len(names) == 0 {
		var out []Host
		for _, h := range Hosts() {
			if h.Detect() {
				out = append(out, h)
			}
		}
		return out, nil
	}
	var out []Host
	for _, name := range names {
		h := HostByKey(name)
		if h == nil {
			return nil, fmt.Errorf("unknown agent host %q — supported: %v", name, AllHostKeys())
		}
		out = append(out, h)
	}
	return out, nil
}
