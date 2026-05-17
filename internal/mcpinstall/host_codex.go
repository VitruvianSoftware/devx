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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	toml "github.com/pelletier/go-toml/v2"
)

// codexHost adapts the OpenAI Codex CLI, which stores its MCP server config
// in TOML under [mcp_servers.<name>] sections of ~/.codex/config.toml.
type codexHost struct{}

func (codexHost) Key() string         { return "codex" }
func (codexHost) DisplayName() string { return "Codex CLI" }

func (codexHost) Detect() bool {
	if _, err := exec.LookPath("codex"); err == nil {
		return true
	}
	if _, err := os.Stat(expandHome("~/.codex")); err == nil {
		return true
	}
	return false
}

func (codexHost) configPath() string {
	return expandHome("~/.codex/config.toml")
}

func (c codexHost) Status(_ Options) (Status, error) {
	st := Status{
		HostKey:     c.Key(),
		DisplayName: c.DisplayName(),
		Detected:    c.Detect(),
		ConfigPath:  c.configPath(),
		Scope:       ScopeUser.String(),
	}
	cfg, err := readTOMLConfig(c.configPath())
	if err != nil {
		st.Note = err.Error()
		return st, nil
	}
	if servers, ok := cfg["mcp_servers"].(map[string]any); ok {
		if entry, ok := servers[ServerEntryKey].(map[string]any); ok {
			st.Installed = true
			if cmd, ok := entry["command"].(string); ok {
				st.Command = cmd
			}
		}
	}
	return st, nil
}

func (c codexHost) Install(opts Options) (InstallResult, error) {
	if opts.Scope == ScopeProject {
		return InstallResult{
			HostKey:     c.Key(),
			DisplayName: c.DisplayName(),
			Message:     "Codex CLI does not support project-scoped MCP config",
		}, fmt.Errorf("project scope unsupported for codex")
	}
	res := InstallResult{
		HostKey:     c.Key(),
		DisplayName: c.DisplayName(),
		ConfigPath:  c.configPath(),
		Scope:       ScopeUser.String(),
	}
	cfg, err := readTOMLConfig(c.configPath())
	if err != nil {
		res.Message = err.Error()
		return res, err
	}
	servers, _ := cfg["mcp_servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	desired := devxEntry(opts)
	existing, hadEntry := servers[ServerEntryKey].(map[string]any)
	if hadEntry && reflect.DeepEqual(existing, desired) {
		res.OK = true
		res.Message = "already up to date"
		return res, nil
	}
	servers[ServerEntryKey] = desired
	cfg["mcp_servers"] = servers
	if opts.DryRun {
		res.OK = true
		res.Changed = true
		res.Message = "would update " + c.configPath()
		return res, nil
	}
	if err := writeTOMLConfig(c.configPath(), cfg); err != nil {
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

func (c codexHost) Uninstall(opts Options) (InstallResult, error) {
	res := InstallResult{
		HostKey:     c.Key(),
		DisplayName: c.DisplayName(),
		ConfigPath:  c.configPath(),
		Scope:       ScopeUser.String(),
	}
	cfg, err := readTOMLConfig(c.configPath())
	if err != nil {
		res.Message = err.Error()
		return res, err
	}
	servers, ok := cfg["mcp_servers"].(map[string]any)
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
	cfg["mcp_servers"] = servers
	if opts.DryRun {
		res.OK = true
		res.Changed = true
		res.Message = "would remove from " + c.configPath()
		return res, nil
	}
	if err := writeTOMLConfig(c.configPath(), cfg); err != nil {
		res.Message = err.Error()
		return res, err
	}
	res.OK = true
	res.Changed = true
	res.Message = "removed devx entry"
	return res, nil
}

func readTOMLConfig(path string) (map[string]any, error) {
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
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s as TOML: %w", path, err)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}

func writeTOMLConfig(path string, cfg map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
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
