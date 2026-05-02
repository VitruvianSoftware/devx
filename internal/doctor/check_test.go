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

package doctor

import "testing"

func TestParseVersion_Podman(t *testing.T) {
	got := parseVersion("podman", "podman version 4.9.4")
	if got != "4.9.4" {
		t.Errorf("expected 4.9.4, got %s", got)
	}
}

func TestParseVersion_Lima(t *testing.T) {
	got := parseVersion("limactl", "limactl version 1.0.2")
	if got != "1.0.2" {
		t.Errorf("expected 1.0.2, got %s", got)
	}
}

func TestParseVersion_Colima(t *testing.T) {
	got := parseVersion("colima", "colima version 0.8.1")
	if got != "0.8.1" {
		t.Errorf("expected 0.8.1, got %s", got)
	}
}

func TestParseVersion_Nerdctl(t *testing.T) {
	got := parseVersion("nerdctl", "nerdctl version 2.0.3")
	if got != "2.0.3" {
		t.Errorf("expected 2.0.3, got %s", got)
	}
}

func TestParseVersion_Cloudflared(t *testing.T) {
	got := parseVersion("cloudflared", "cloudflared version 2024.12.1 (built 2024-12-18-1111 lgo)")
	if got != "2024.12.1" {
		t.Errorf("expected 2024.12.1, got %s", got)
	}
}

func TestParseVersion_Docker(t *testing.T) {
	got := parseVersion("docker", "Docker version 27.4.0, build abcdef")
	if got != "27.4.0" {
		t.Errorf("expected 27.4.0, got %s", got)
	}
}

func TestParseVersion_Kubectl(t *testing.T) {
	got := parseVersion("kubectl", "Client Version: v1.31.0")
	if got != "v1.31.0" {
		t.Errorf("expected v1.31.0, got %s", got)
	}
}

func TestParseVersion_Fallback(t *testing.T) {
	got := parseVersion("unknown-tool", "some version string")
	if got != "some version string" {
		t.Errorf("expected fallback to first line, got %s", got)
	}
}

func TestParseVersion_LongFallback(t *testing.T) {
	long := "this is a very long version string that exceeds forty characters in total"
	got := parseVersion("unknown-tool", long)
	if len(got) > 44 { // 40 chars + "..."
		t.Errorf("expected truncated fallback, got %s (len %d)", got, len(got))
	}
}

func TestAllTools_PodmanNotRequired(t *testing.T) {
	tools := allTools()
	for _, tool := range tools {
		if tool.Binary == "podman" {
			if tool.Required {
				t.Error("podman should not be marked as Required — it is now one of several VM backends")
			}
			return
		}
	}
	t.Error("podman not found in allTools()")
}

func TestAllTools_ContainsLima(t *testing.T) {
	tools := allTools()
	found := false
	for _, tool := range tools {
		if tool.Binary == "limactl" {
			found = true
			if tool.Required {
				t.Error("lima should not be required")
			}
			if tool.InstallBrew != "lima" {
				t.Errorf("expected brew package 'lima', got %s", tool.InstallBrew)
			}
		}
	}
	if !found {
		t.Error("limactl not found in allTools()")
	}
}

func TestAllTools_ContainsColima(t *testing.T) {
	tools := allTools()
	found := false
	for _, tool := range tools {
		if tool.Binary == "colima" {
			found = true
			if tool.Required {
				t.Error("colima should not be required")
			}
		}
	}
	if !found {
		t.Error("colima not found in allTools()")
	}
}

func TestAllTools_ContainsNerdctl(t *testing.T) {
	tools := allTools()
	found := false
	for _, tool := range tools {
		if tool.Binary == "nerdctl" {
			found = true
			if tool.FeatureArea != "Container Runtime" {
				t.Errorf("expected feature area 'Container Runtime', got %s", tool.FeatureArea)
			}
		}
	}
	if !found {
		t.Error("nerdctl not found in allTools()")
	}
}

func TestDetectSystem(t *testing.T) {
	info := DetectSystem()
	if info.OS == "" {
		t.Error("OS should not be empty")
	}
	if info.Arch == "" {
		t.Error("Arch should not be empty")
	}
}
