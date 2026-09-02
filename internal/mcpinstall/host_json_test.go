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
	"os"
	"path/filepath"
	"testing"
)

// newTestJSONHost builds a jsonHost rooted in tmpDir so each test gets its
// own isolated config file. We only exercise project scope here because user
// scope is just a different path resolution — same write logic.
func newTestJSONHost(tmpDir, projectFile, containerKey string) *jsonHost {
	return &jsonHost{
		displayName:   "Test Host",
		key:           "test-host",
		projectConfig: projectFile,
		userConfig:    filepath.Join(tmpDir, "user.json"),
		containerKey:  containerKey,
	}
}

func TestJSONInstallCreatesNewConfig(t *testing.T) {
	dir := t.TempDir()
	h := newTestJSONHost(dir, "config.json", "")
	opts := Options{ProjectDir: dir, Scope: ScopeProject, DevxBinary: "/usr/local/bin/devx"}

	res, err := h.Install(opts)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !res.OK || !res.Changed {
		t.Errorf("expected OK+Changed, got %+v", res)
	}

	cfg := readBack(t, filepath.Join(dir, "config.json"))
	servers, _ := cfg["mcpServers"].(map[string]any)
	if servers == nil {
		t.Fatalf("mcpServers missing: %+v", cfg)
		return
	}
	entry, _ := servers["devx"].(map[string]any)
	if entry == nil {
		t.Fatalf("devx entry missing: %+v", servers)
		return
	}
	if entry["command"] != "/usr/local/bin/devx" {
		t.Errorf("command = %v, want /usr/local/bin/devx", entry["command"])
	}
	args, _ := entry["args"].([]any)
	if len(args) != 2 || args[0] != "mcp" || args[1] != "serve" {
		t.Errorf("args = %v, want [mcp serve]", args)
	}
}

func TestJSONInstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	h := newTestJSONHost(dir, "config.json", "")
	opts := Options{ProjectDir: dir, Scope: ScopeProject, DevxBinary: "/usr/local/bin/devx"}

	first, err := h.Install(opts)
	if err != nil || !first.Changed {
		t.Fatalf("first install: %+v err=%v", first, err)
	}
	second, err := h.Install(opts)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if second.Changed {
		t.Errorf("second install should be a no-op, got Changed=true: %+v", second)
	}
	if !second.OK {
		t.Errorf("second install should report OK")
	}
}

func TestJSONInstallPreservesOtherEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Pre-existing config the user owns.
	original := map[string]any{
		"theme":     "dark",
		"telemetry": true,
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "/usr/bin/other",
				"args":    []any{"--flag"},
			},
		},
	}
	writeJSON(t, path, original)

	h := newTestJSONHost(dir, "config.json", "")
	if _, err := h.Install(Options{ProjectDir: dir, Scope: ScopeProject, DevxBinary: "/usr/local/bin/devx"}); err != nil {
		t.Fatal(err)
	}

	cfg := readBack(t, path)
	if cfg["theme"] != "dark" {
		t.Errorf("user setting 'theme' was wiped: %v", cfg["theme"])
	}
	if cfg["telemetry"] != true {
		t.Errorf("user setting 'telemetry' was wiped: %v", cfg["telemetry"])
	}
	servers, _ := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["other-server"]; !ok {
		t.Errorf("other MCP server entry was wiped: %+v", servers)
	}
	if _, ok := servers["devx"]; !ok {
		t.Errorf("devx entry was not added: %+v", servers)
	}
}

func TestJSONUninstallRemovesOnlyDevx(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	writeJSON(t, path, map[string]any{
		"mcpServers": map[string]any{
			"devx": map[string]any{"command": "/x", "args": []any{"mcp", "serve"}},
			"other-server": map[string]any{
				"command": "/usr/bin/other",
				"args":    []any{"--flag"},
			},
		},
	})

	h := newTestJSONHost(dir, "config.json", "")
	res, err := h.Uninstall(Options{ProjectDir: dir, Scope: ScopeProject})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Errorf("expected change, got %+v", res)
	}

	cfg := readBack(t, path)
	servers, _ := cfg["mcpServers"].(map[string]any)
	if _, ok := servers["devx"]; ok {
		t.Errorf("devx entry should be removed: %+v", servers)
	}
	if _, ok := servers["other-server"]; !ok {
		t.Errorf("other-server should be preserved: %+v", servers)
	}
}

func TestJSONUninstallNoopWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	h := newTestJSONHost(dir, "config.json", "")
	res, err := h.Uninstall(Options{ProjectDir: dir, Scope: ScopeProject})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Errorf("uninstall of missing config should be a no-op")
	}
	if !res.OK {
		t.Errorf("uninstall should still report OK")
	}
}

func TestJSONStatusReflectsInstallState(t *testing.T) {
	dir := t.TempDir()
	h := newTestJSONHost(dir, "config.json", "")
	opts := Options{ProjectDir: dir, Scope: ScopeProject, DevxBinary: "/usr/local/bin/devx"}

	st, _ := h.Status(opts)
	if st.Installed {
		t.Errorf("status should be uninstalled before Install: %+v", st)
	}

	if _, err := h.Install(opts); err != nil {
		t.Fatal(err)
	}

	st, _ = h.Status(opts)
	if !st.Installed {
		t.Errorf("status should show installed after Install: %+v", st)
	}
	if st.Command != "/usr/local/bin/devx" {
		t.Errorf("status.Command = %q", st.Command)
	}
}

func TestJSONCustomContainerKey(t *testing.T) {
	dir := t.TempDir()
	h := newTestJSONHost(dir, "config.json", "context_servers")
	opts := Options{ProjectDir: dir, Scope: ScopeProject, DevxBinary: "/x"}

	if _, err := h.Install(opts); err != nil {
		t.Fatal(err)
	}
	cfg := readBack(t, filepath.Join(dir, "config.json"))
	if _, ok := cfg["context_servers"]; !ok {
		t.Errorf("custom container key 'context_servers' missing: %+v", cfg)
	}
	if _, ok := cfg["mcpServers"]; ok {
		t.Errorf("default 'mcpServers' should not be used when custom key set")
	}
}

func TestJSONInstallDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	h := newTestJSONHost(dir, "config.json", "")
	res, err := h.Install(Options{
		ProjectDir: dir,
		Scope:      ScopeProject,
		DryRun:     true,
		DevxBinary: "/x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Errorf("dry-run should report Changed=true (would write)")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote a file: %v", err)
	}
}

func TestJSONProjectScopeErrorsWhenUnsupported(t *testing.T) {
	dir := t.TempDir()
	h := &jsonHost{
		key:        "user-only",
		userConfig: filepath.Join(dir, "user.json"),
		// No projectConfig — project scope should fail.
	}
	_, err := h.Install(Options{ProjectDir: dir, Scope: ScopeProject})
	if err == nil {
		t.Errorf("expected error when project scope unsupported")
	}
}

func TestJSONAutoScopePrefersProjectWhenSupported(t *testing.T) {
	dir := t.TempDir()
	h := newTestJSONHost(dir, "config.json", "")
	res, err := h.Install(Options{ProjectDir: dir, Scope: ScopeAuto, DevxBinary: "/x"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Scope != ScopeProject.String() {
		t.Errorf("auto scope should pick project, got %q", res.Scope)
	}
}

func writeJSON(t *testing.T, path string, cfg map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readBack(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return cfg
}
