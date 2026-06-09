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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderBuilderProvision_Golden(t *testing.T) {
	got := RenderBuilderProvision()
	golden := filepath.Join("testdata", "builder-provision.sh.golden")
	if *update {
		_ = os.WriteFile(golden, []byte(got), 0o644)
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run -update): %v", err)
	}
	if got != string(want) {
		t.Errorf("provision script mismatch\n--- got ---\n%s", got)
	}
}

func TestRenderBuilderProvision_Tools(t *testing.T) {
	s := RenderBuilderProvision()
	for _, tool := range []string{"podman", "wget", "butane"} {
		if !strings.Contains(s, tool) {
			t.Errorf("provision script must install %q", tool)
		}
	}
	// Ventoy/exFAT/partition tooling is no longer used.
	for _, gone := range []string{"ventoy", "exfatprogs", "parted", "jq"} {
		if strings.Contains(s, gone) {
			t.Errorf("provision script must not install Ventoy-era tool %q", gone)
		}
	}
	if strings.Contains(s, "coreos-installer") {
		t.Error("coreos-installer ships no Linux binary; run it via the container, not apt")
	}
}

func TestBuildImageCommands(t *testing.T) {
	var calls [][]string
	fake := func(ctx context.Context, name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name}, args...))
		return "", nil
	}
	b := &Builder{VMName: "devx-usb-builder", run: fake}
	_, err := b.BuildImage(context.Background(), AssemblyParams{
		ImageSizeMB: 4096,
		ButaneVM:    "/tmp/devx/payload/fcos/ephemeral/devx-join.bu",
		ImageVM:     "/tmp/devx/devx-usb.img",
	}, "/local/staging")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, c := range calls {
		joined += strings.Join(c, " ") + "\n"
	}
	for _, want := range []string{
		"limactl shell devx-usb-builder",
		"limactl copy",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}
