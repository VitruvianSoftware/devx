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

package projectcfg

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDevxYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "devx.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadSettings_ParsesEnvAndBridge(t *testing.T) {
	t.Chdir(writeDevxYAML(t, "env:\n  - file://.env\n  - op://vault/item\nai:\n  bridge: false\n"))

	s, found := LoadSettings()
	if !found {
		t.Fatal("LoadSettings should find devx.yaml")
	}
	if len(s.Env) != 2 || s.Env[1] != "op://vault/item" {
		t.Errorf("env not parsed: %v", s.Env)
	}
	if s.AIBridgeEnabled() {
		t.Error("`ai: bridge: false` must disable the bridge")
	}
}

func TestLoadSettings_DefaultsEnvToDotEnv(t *testing.T) {
	t.Chdir(writeDevxYAML(t, "services:\n  api:\n    port: 8080\n"))

	s, found := LoadSettings()
	if !found {
		t.Fatal("LoadSettings should find devx.yaml")
	}
	if len(s.Env) != 1 || s.Env[0] != "file://.env" {
		t.Errorf("env should default to [file://.env], got %v", s.Env)
	}
	if !s.AIBridgeEnabled() {
		t.Error("no `ai.bridge` → bridge enabled by default")
	}
}

func TestLoadSettings_NoConfig_FallsBack(t *testing.T) {
	// A bare temp dir whose ancestors have no devx.yaml → found=false, no error.
	t.Chdir(t.TempDir())
	if _, found := LoadSettings(); found {
		t.Error("LoadSettings must report not-found when there is no devx.yaml to fall back to .env")
	}
}

func TestLoad_UnmarshalsArbitraryStruct(t *testing.T) {
	t.Chdir(writeDevxYAML(t, "name: demo\n"))
	var out struct {
		Name string `yaml:"name"`
	}
	if err := Load(&out); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out.Name != "demo" {
		t.Errorf("Load did not unmarshal: %+v", out)
	}
}
