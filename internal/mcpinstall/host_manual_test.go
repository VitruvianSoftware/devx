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
	"strings"
	"testing"
)

func TestManualInstallEmitsSnippetWithoutTouchingDisk(t *testing.T) {
	h := &manualHost{
		key:          "test-manual",
		displayName:  "Test Manual Host",
		instructions: "Paste this into Settings → MCP.",
	}
	res, err := h.Install(Options{DevxBinary: "/usr/local/bin/devx"})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("manual install should report OK")
	}
	if res.Changed {
		t.Errorf("manual install should never report Changed (no file written)")
	}
	if !strings.Contains(res.Message, "Paste this into Settings") {
		t.Errorf("Message should include instructions: %s", res.Message)
	}
	if !strings.Contains(res.Message, "\"command\": \"/usr/local/bin/devx\"") {
		t.Errorf("snippet should include devx command path: %s", res.Message)
	}
	if !strings.Contains(res.Message, "\"mcp\"") || !strings.Contains(res.Message, "\"serve\"") {
		t.Errorf("snippet should include args: %s", res.Message)
	}
}

func TestManualStatusReportsManualScope(t *testing.T) {
	h := &manualHost{key: "x", displayName: "X"}
	st, _ := h.Status(Options{})
	if st.Scope != "manual" {
		t.Errorf("scope = %q, want manual", st.Scope)
	}
	if st.Installed {
		t.Errorf("manual host should never report Installed=true (cannot detect)")
	}
}

func TestManualUninstallIsAdvisory(t *testing.T) {
	h := &manualHost{key: "x", displayName: "X"}
	res, _ := h.Uninstall(Options{})
	if !res.OK {
		t.Errorf("manual uninstall should report OK")
	}
	if !strings.Contains(res.Message, "manually") {
		t.Errorf("uninstall message should advise manual removal: %s", res.Message)
	}
}
