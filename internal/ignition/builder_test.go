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

package ignition_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/ignition"
)

// makeTemplate writes a minimal valid Butane template to a temp file.
func makeTemplate(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.bu")
	if err != nil {
		t.Fatalf("creating temp template: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing template: %v", err)
	}
	f.Close()
	return f.Name()
}

// minimalTemplate is a valid Butane config with variable placeholders.
const minimalTemplate = `variant: fcos
version: 1.5.0
storage:
  files:
    - path: /etc/test.conf
      mode: 0644
      contents:
        inline: |
          HOST=${DEV_HOSTNAME}
          TOKEN=${CF_TUNNEL_TOKEN}
`

func TestBuild_SubstitutesVariables(t *testing.T) {
	if _, err := exec.LookPath("butane"); err != nil {
		t.Skip("butane not in PATH — skipping integration test")
	}

	tmplPath := makeTemplate(t, minimalTemplate)

	ignPath, err := ignition.Build(tmplPath, "test-token-123", "dummy-id", "my-test-machine", "test.dev", "podman")
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	defer os.Remove(ignPath)

	data, err := os.ReadFile(ignPath)
	if err != nil {
		t.Fatalf("reading ignition output: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "test-token-123") {
		t.Error("ignition output missing substituted CF_TUNNEL_TOKEN")
	}
	if !strings.Contains(content, "my-test-machine") {
		t.Error("ignition output missing substituted DEV_HOSTNAME")
	}
}

func TestBuild_MissingTemplate(t *testing.T) {
	_, err := ignition.Build("/nonexistent/path/template.bu", "token", "id", "host", "dev", "podman")
	if err == nil {
		t.Error("expected error for missing template, got nil")
	}
}

func TestBuild_WritesToTempFile(t *testing.T) {
	if _, err := exec.LookPath("butane"); err != nil {
		t.Skip("butane not in PATH — skipping integration test")
	}

	tmplPath := makeTemplate(t, minimalTemplate)
	ignPath, err := ignition.Build(tmplPath, "tok", "id", "host", "dev", "podman")
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	defer os.Remove(ignPath)

	// Should be a real file in the OS temp directory
	if !strings.HasPrefix(ignPath, os.TempDir()) {
		if filepath.Dir(ignPath) == "." {
			t.Error("ignition file is not in a temp directory")
		}
	}
	if _, err := os.Stat(ignPath); err != nil {
		t.Errorf("ignition output file does not exist: %v", err)
	}
}

func TestValidate_WithDummyValues(t *testing.T) {
	if _, err := exec.LookPath("butane"); err != nil {
		t.Skip("butane not in PATH — skipping integration test")
	}

	tmplPath := makeTemplate(t, minimalTemplate)
	if err := ignition.Validate(tmplPath); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}
