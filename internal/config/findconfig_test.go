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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectConfig_InCurrentDir(t *testing.T) {
	tmpDir := t.TempDir()

	configName := "devx.yaml"
	configPath := filepath.Join(tmpDir, configName)

	if err := os.WriteFile(configPath, []byte("name: test"), 0644); err != nil {
		t.Fatalf("failed to create dummy config: %v", err)
	}

	foundPath, foundDir, err := FindProjectConfig(tmpDir, configName)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if foundPath != configPath {
		t.Errorf("expected path %q, got %q", configPath, foundPath)
	}
	if foundDir != tmpDir {
		t.Errorf("expected dir %q, got %q", tmpDir, foundDir)
	}
}

func TestFindProjectConfig_InParentDir(t *testing.T) {
	tmpDir := t.TempDir()

	configName := "devx.yaml"
	configPath := filepath.Join(tmpDir, configName)

	if err := os.WriteFile(configPath, []byte("name: test"), 0644); err != nil {
		t.Fatalf("failed to create dummy config: %v", err)
	}

	deepDir := filepath.Join(tmpDir, "src", "nested", "module")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	foundPath, foundDir, err := FindProjectConfig(deepDir, configName)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if foundPath != configPath {
		t.Errorf("expected path %q, got %q", configPath, foundPath)
	}
	if foundDir != tmpDir {
		t.Errorf("expected dir %q, got %q", tmpDir, foundDir)
	}
}

func TestFindProjectConfig_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	deepDir := filepath.Join(tmpDir, "src", "nested")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	// Make sure devx.yaml doesn't exist anywhere in the hierarchy
	_, _, err := FindProjectConfig(deepDir, "non_existent_file.yaml")
	if err != ErrConfigNotFound {
		t.Fatalf("expected ErrConfigNotFound, got %v", err)
	}
}

func TestFindProjectConfig_ReturnsAbsPath(t *testing.T) {
	tmpDir := t.TempDir()

	configName := "devx.yaml"
	configPath := filepath.Join(tmpDir, configName)

	if err := os.WriteFile(configPath, []byte("name: test"), 0644); err != nil {
		t.Fatalf("failed to create dummy config: %v", err)
	}

	// Use a relative start dir by changing into tmpDir
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	foundPath, foundDir, err := FindProjectConfig(".", configName)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !filepath.IsAbs(foundPath) {
		t.Errorf("expected absolute path, got %q", foundPath)
	}
	if !filepath.IsAbs(foundDir) {
		t.Errorf("expected absolute dir, got %q", foundDir)
	}
}
