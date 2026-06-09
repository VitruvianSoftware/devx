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

package usb

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// memSink captures artifacts in memory for assertions.
type memSink struct{ files map[string][]byte }

func (m *memSink) Add(a Artifact) error {
	if m.files == nil {
		m.files = map[string][]byte{}
	}
	m.files[a.Path] = a.Contents
	return nil
}

func TestFCOSRenderer(t *testing.T) {
	s := agentSpec()
	sink := &memSink{}
	entries, err := FCOSRenderer{}.Render(s, ModeEphemeral, sink)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Renderer != "fcos" || e.BaseImage != FCOSStickName || e.Injection.Kind != InjectIgnition {
		t.Errorf("unexpected entry: %+v", e)
	}
	if e.InstallToDisk {
		t.Error("ephemeral entry must not install to disk")
	}

	bu, ok := sink.files["fcos/ephemeral/devx-join.bu"]
	if !ok {
		t.Fatal("missing butane artifact")
	}
	if !strings.Contains(string(bu), "variant: fcos") || !strings.Contains(string(bu), "devx-join.service") {
		t.Error("butane must declare fcos variant and the join unit")
	}
	// The embedded script must be the base64 of the canonical join script.
	script, _ := RenderJoinScript(withMode(s, ModeEphemeral))
	wantB64 := base64.StdEncoding.EncodeToString([]byte(script))
	if !strings.Contains(string(bu), wantB64) {
		t.Error("butane must embed the canonical join script as base64")
	}

	// Install mode adds the disk-install unit and marks InstallToDisk.
	sink2 := &memSink{}
	es, err := FCOSRenderer{}.Render(s, ModeInstall, sink2)
	if err != nil {
		t.Fatal(err)
	}
	if !es[0].InstallToDisk {
		t.Error("install entry must set InstallToDisk")
	}
	if !strings.Contains(string(sink2.files["fcos/install/devx-join.bu"]), "tailscaled.service") {
		t.Error("butane must provision tailscaled (static binary) for the tailnet")
	}
}

func TestBakedRenderer(t *testing.T) {
	s := agentSpec()
	sink := &memSink{}
	entries, err := BakedRenderer{}.Render(s, ModeEphemeral, sink)
	if err != nil {
		t.Fatal(err)
	}
	if entries[0].Injection.Kind != InjectConfigPartition || entries[0].BaseImage != BakedImage {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
	if _, ok := sink.files["baked/ephemeral/devx-config/devx-join.sh"]; !ok {
		t.Error("baked must emit the join script onto the config partition")
	}
	if got := string(sink.files["baked/ephemeral/devx-config/mode"]); got != "ephemeral\n" {
		t.Errorf("mode file = %q, want ephemeral", got)
	}
}

func TestDirSink(t *testing.T) {
	root := t.TempDir()
	sink := DirSink{Root: root}
	if err := sink.Add(Artifact{Path: "a/b/c.txt", Contents: []byte("hi"), Mode: 0o600}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "a", "b", "c.txt"))
	if err != nil || string(got) != "hi" {
		t.Fatalf("file not written correctly: %q %v", got, err)
	}
	if err := sink.Add(Artifact{Path: "../escape", Contents: []byte("x")}); err == nil {
		t.Error("DirSink must reject paths containing ..")
	}
}

func withMode(s JoinSpec, m Mode) JoinSpec {
	s.Mode = m
	return s
}
