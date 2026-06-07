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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleAssembly() AssemblyParams {
	return AssemblyParams{
		BootSizeMB:  16384,
		TotalSizeMB: 117000,
		ISOs: []ISOSpec{
			{Name: "fcos", URL: "https://example/fcos.iso", Filename: "fcos.iso", EmbedIgnition: true},
			{Name: "ubuntu", URL: "https://example/ubuntu.iso", Filename: "ubuntu.iso"},
		},
		ButaneVM:  "/tmp/devx/payload/fcos.ign",
		PayloadVM: "/tmp/devx/payload",
		ImageVM:   "/tmp/devx/devx-usb.img",
	}
}

func TestRenderAssemblyScript_Golden(t *testing.T) {
	got, err := RenderAssemblyScript(sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "assemble.sh.golden")
	if *update {
		_ = os.WriteFile(golden, []byte(got), 0o644)
		return
	}
	want, _ := os.ReadFile(golden)
	if got != string(want) {
		t.Errorf("assembly script mismatch\n--- got ---\n%s", got)
	}
}

func TestRenderAssemblyScript_Invariants(t *testing.T) {
	got, err := RenderAssemblyScript(sampleAssembly())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"wget -q -O",
		"butane --strict -o /tmp/devx-node.ign",
		"iso ignition embed",            // via the coreos-installer container
		"Ventoy2Disk.sh -I -r 100616",   // reserve = total-boot = 117000-16384
		"mkfs.exfat -L DEVXDATA",        // exFAT storage with the right label flag
		"truncate -s 117000M",           // image size
		"trap ",                         // loop/mount cleanup on failure
		"[ -b \"${LOOP}p3\" ] && break", // wait for the partition node
	} {
		if !strings.Contains(got, want) {
			t.Errorf("assembly script must contain %q", want)
		}
	}
	if strings.Contains(got, "coreos-installer install") || strings.Contains(got, "/dev/sda") {
		t.Error("assembly must not write to any physical/host disk")
	}
	if strings.Contains(got, "mkfs.exfat -n") {
		t.Error("must use exfatprogs -L label flag, not the legacy -n")
	}
}

func TestRenderAssemblyScript_RequiresFCOS(t *testing.T) {
	p := sampleAssembly()
	p.ISOs = []ISOSpec{{Name: "ubuntu", URL: "u", Filename: "u.iso"}}
	if _, err := RenderAssemblyScript(p); err == nil {
		t.Error("an FCOS ISO with EmbedIgnition is required")
	}
}

func TestDefaultISOSpecs(t *testing.T) {
	specs := DefaultISOSpecs([]string{"fcos", "ubuntu"})
	if len(specs) != 2 {
		t.Fatalf("want 2 specs, got %d", len(specs))
	}
	if !specs[0].EmbedIgnition || specs[0].Resolver != "fcos" {
		t.Errorf("fcos spec should embed ignition + resolve via stream: %+v", specs[0])
	}
	if specs[1].Resolver != "ubuntu" || specs[1].URL != "" {
		t.Errorf("ubuntu spec should resolve at runtime (no pinned URL): %+v", specs[1])
	}
	// The resolvers must emit their runtime resolution (FCOS stream + jq, Ubuntu dir scrape).
	got, err := RenderAssemblyScript(AssemblyParams{
		BootSizeMB: 1024, TotalSizeMB: 4096, ISOs: specs,
		ButaneVM: "/i.ign", PayloadVM: "/p", ImageVM: "/o.img",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, FCOSStreamURL) || !strings.Contains(got, "jq -r") {
		t.Error("FCOS spec must resolve the ISO via stream metadata + jq")
	}
	if !strings.Contains(got, UbuntuReleasesDir) || !strings.Contains(got, "live-server-amd64") {
		t.Error("Ubuntu spec must resolve the latest ISO from the releases dir")
	}
}
