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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
)

// ServerEntryKey is the conventional name devx registers itself under in
// every host's MCP config. Stable string used for idempotency checks across
// installs.
const ServerEntryKey = "devx"

// jsonHost is the default adapter for hosts whose MCP config is a JSON file
// with a top-level map keyed by server name. Covers Claude Code, Cursor,
// Gemini CLI, Continue, Windsurf, OpenCode, and Zed (with key=context_servers).
type jsonHost struct {
	displayName string
	key         string

	// detectBins are checked via exec.LookPath. Finding any one is a hit.
	detectBins []string
	// detectPaths are absolute paths (after $HOME expansion) whose existence
	// indicates installation. Used when the binary is bundled (macOS apps,
	// VSCode-style data dirs).
	detectPaths []string

	// projectConfig is the path relative to the project root, e.g.
	// ".mcp.json" or ".cursor/mcp.json". Empty = project scope not supported.
	projectConfig string
	// userConfig is an absolute path with $HOME expansion already applied.
	userConfig string

	// containerKey is the top-level JSON key under which MCP servers live.
	// Defaults to "mcpServers" (Claude Code / Cursor convention). Zed uses
	// "context_servers".
	containerKey string
}

func (h *jsonHost) Key() string         { return h.key }
func (h *jsonHost) DisplayName() string { return h.displayName }

func (h *jsonHost) Detect() bool {
	for _, bin := range h.detectBins {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	for _, p := range h.detectPaths {
		expanded := expandHome(p)
		if _, err := os.Stat(expanded); err == nil {
			return true
		}
	}
	return false
}

func (h *jsonHost) container() string {
	if h.containerKey == "" {
		return "mcpServers"
	}
	return h.containerKey
}

// resolveConfigPath picks the file path based on the requested scope. Errors
// when project scope is requested but not supported by this host.
func (h *jsonHost) resolveConfigPath(opts Options) (string, Scope, error) {
	scope := opts.Scope
	if scope == ScopeAuto {
		if h.projectConfig != "" {
			scope = ScopeProject
		} else {
			scope = ScopeUser
		}
	}
	switch scope {
	case ScopeProject:
		if h.projectConfig == "" {
			return "", scope, fmt.Errorf("host %q does not support project-scoped MCP config", h.key)
		}
		root := opts.ProjectDir
		if root == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return "", scope, fmt.Errorf("resolving project dir: %w", err)
			}
			root = cwd
		}
		return filepath.Join(root, h.projectConfig), scope, nil
	case ScopeUser:
		if h.userConfig == "" {
			return "", scope, fmt.Errorf("host %q has no user-scoped MCP config defined", h.key)
		}
		return expandHome(h.userConfig), scope, nil
	}
	return "", scope, fmt.Errorf("unknown scope %v", opts.Scope)
}

// devxEntry constructs the MCP server entry that gets written under
// containerKey.ServerEntryKey.
func devxEntry(opts Options) map[string]any {
	bin := opts.DevxBinary
	if bin == "" {
		if exe, err := os.Executable(); err == nil {
			bin = exe
		} else {
			bin = "devx"
		}
	}
	return map[string]any{
		"command": bin,
		"args":    []any{"mcp", "serve"},
	}
}

func (h *jsonHost) Status(opts Options) (Status, error) {
	st := Status{
		HostKey:     h.key,
		DisplayName: h.displayName,
		Detected:    h.Detect(),
	}
	path, scope, err := h.resolveConfigPath(opts)
	if err != nil {
		st.Note = err.Error()
		return st, nil
	}
	st.ConfigPath = path
	st.Scope = scope.String()

	cfg, err := readJSONConfig(path)
	if err != nil {
		st.Note = err.Error()
		return st, nil
	}
	if servers, ok := cfg[h.container()].(map[string]any); ok {
		if entry, ok := servers[ServerEntryKey].(map[string]any); ok {
			st.Installed = true
			if cmd, ok := entry["command"].(string); ok {
				st.Command = cmd
			}
		}
	}
	return st, nil
}

func (h *jsonHost) Install(opts Options) (InstallResult, error) {
	res := InstallResult{
		HostKey:     h.key,
		DisplayName: h.displayName,
	}
	path, scope, err := h.resolveConfigPath(opts)
	if err != nil {
		res.Message = err.Error()
		return res, err
	}
	res.ConfigPath = path
	res.Scope = scope.String()

	cfg, err := readJSONConfig(path)
	if err != nil {
		res.Message = err.Error()
		return res, err
	}

	servers, _ := cfg[h.container()].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	desired := devxEntry(opts)
	existing, hadEntry := servers[ServerEntryKey].(map[string]any)
	if hadEntry && reflect.DeepEqual(existing, desired) {
		res.OK = true
		res.Changed = false
		res.Message = "already up to date"
		return res, nil
	}
	servers[ServerEntryKey] = desired
	cfg[h.container()] = servers

	if opts.DryRun {
		res.OK = true
		res.Changed = true
		res.Message = "would update " + path
		return res, nil
	}
	if err := writeJSONConfig(path, cfg); err != nil {
		res.Message = err.Error()
		return res, err
	}
	res.OK = true
	res.Changed = true
	if hadEntry {
		res.Message = "updated existing entry"
	} else {
		res.Message = "added devx MCP entry"
	}
	return res, nil
}

func (h *jsonHost) Uninstall(opts Options) (InstallResult, error) {
	res := InstallResult{
		HostKey:     h.key,
		DisplayName: h.displayName,
	}
	path, scope, err := h.resolveConfigPath(opts)
	if err != nil {
		res.Message = err.Error()
		return res, err
	}
	res.ConfigPath = path
	res.Scope = scope.String()

	cfg, err := readJSONConfig(path)
	if err != nil {
		res.Message = err.Error()
		return res, err
	}
	servers, ok := cfg[h.container()].(map[string]any)
	if !ok {
		res.OK = true
		res.Message = "nothing to remove"
		return res, nil
	}
	if _, present := servers[ServerEntryKey]; !present {
		res.OK = true
		res.Message = "nothing to remove"
		return res, nil
	}
	delete(servers, ServerEntryKey)
	if len(servers) == 0 {
		// Leave the empty mcpServers key in place — some hosts treat its
		// absence as "no MCP support enabled."
		cfg[h.container()] = servers
	}

	if opts.DryRun {
		res.OK = true
		res.Changed = true
		res.Message = "would remove from " + path
		return res, nil
	}
	if err := writeJSONConfig(path, cfg); err != nil {
		res.Message = err.Error()
		return res, err
	}
	res.OK = true
	res.Changed = true
	res.Message = "removed devx entry"
	return res, nil
}

// readJSONConfig loads the file as a JSON object, returning an empty map if
// the file does not yet exist. Other I/O errors propagate.
func readJSONConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s as JSON: %w", path, err)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}

// writeJSONConfig serializes cfg with 2-space indent and atomically replaces
// the target file (temp + rename). Creates parent directories as needed.
func writeJSONConfig(path string, cfg map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	// Trailing newline matches what most editors produce.
	data = append(data, '\n')
	tmp := path + ".devx-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}

// expandHome turns a leading "~/" into the user's home directory. Other
// paths are returned unchanged.
func expandHome(p string) string {
	if len(p) >= 2 && p[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
