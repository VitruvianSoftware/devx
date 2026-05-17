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
	"os"
	"path/filepath"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

// codexHost reads ~/.codex/config.toml unconditionally — to test it we
// redirect $HOME for the duration of the test so the adapter writes to a
// tempdir instead of the real user config.
func withFakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestCodexInstallRoundTrip(t *testing.T) {
	home := withFakeHome(t)
	host := codexHost{}

	res, err := host.Install(Options{DevxBinary: "/usr/local/bin/devx"})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !res.Changed || !res.OK {
		t.Errorf("expected OK+Changed, got %+v", res)
	}

	data, err := os.ReadFile(filepath.Join(home, ".codex/config.toml"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var cfg map[string]any
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse TOML: %v", err)
	}
	servers, _ := cfg["mcp_servers"].(map[string]any)
	if servers == nil {
		t.Fatalf("[mcp_servers] missing: %+v", cfg)
	}
	devx, _ := servers["devx"].(map[string]any)
	if devx == nil {
		t.Fatalf("[mcp_servers.devx] missing: %+v", servers)
	}
	if devx["command"] != "/usr/local/bin/devx" {
		t.Errorf("command = %v", devx["command"])
	}
}

func TestCodexInstallIdempotent(t *testing.T) {
	withFakeHome(t)
	host := codexHost{}

	first, err := host.Install(Options{DevxBinary: "/x"})
	if err != nil || !first.Changed {
		t.Fatalf("first: %+v err=%v", first, err)
	}
	second, err := host.Install(Options{DevxBinary: "/x"})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Changed {
		t.Errorf("second install should no-op, got %+v", second)
	}
}

func TestCodexInstallPreservesOtherServersAndKeys(t *testing.T) {
	home := withFakeHome(t)
	path := filepath.Join(home, ".codex/config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Hand-write a TOML with an existing mcp_servers entry and a
	// top-level key the user owns.
	original := `model = "gpt-5"

[mcp_servers.legacy]
command = "/usr/bin/legacy"
args = ["serve"]
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	host := codexHost{}
	if _, err := host.Install(Options{DevxBinary: "/x"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var cfg map[string]any
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["model"] != "gpt-5" {
		t.Errorf("user 'model' key was wiped: %v", cfg["model"])
	}
	servers, _ := cfg["mcp_servers"].(map[string]any)
	if _, ok := servers["legacy"]; !ok {
		t.Errorf("legacy mcp server was wiped: %+v", servers)
	}
	if _, ok := servers["devx"]; !ok {
		t.Errorf("devx entry was not added: %+v", servers)
	}
}

func TestCodexRejectsProjectScope(t *testing.T) {
	withFakeHome(t)
	host := codexHost{}
	_, err := host.Install(Options{Scope: ScopeProject})
	if err == nil {
		t.Errorf("expected error when project scope requested for Codex")
	}
}

func TestCodexUninstallRemovesEntry(t *testing.T) {
	withFakeHome(t)
	host := codexHost{}
	if _, err := host.Install(Options{DevxBinary: "/x"}); err != nil {
		t.Fatal(err)
	}
	res, err := host.Uninstall(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Errorf("expected Changed=true for uninstall")
	}
	st, _ := host.Status(Options{})
	if st.Installed {
		t.Errorf("status should report uninstalled, got %+v", st)
	}
}

func TestCodexUninstallNoopWhenAbsent(t *testing.T) {
	withFakeHome(t)
	host := codexHost{}
	res, err := host.Uninstall(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Errorf("expected no-op when config is absent")
	}
	if !res.OK {
		t.Errorf("expected OK=true even when nothing to remove")
	}
}
