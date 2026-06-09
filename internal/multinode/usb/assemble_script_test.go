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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleAssembly() AssemblyParams {
	return AssemblyParams{
		ImageSizeMB: 4096,
		ButaneVM:    "/tmp/devx/payload/fcos/ephemeral/devx-join.bu",
		ImageVM:     "/tmp/devx/devx-usb.img",
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
		`butane --strict -o "$WORK/devx-node.ign"`,            // compile join Butane → Ignition
		"truncate -s 4096M",                                   // sparse image sized for the FCOS metal layout
		`LOOP=$(losetup --show -f -P "$IMG")`,                 // loop device with partition scanning
		"trap ",                                               // loop cleanup on failure
		"podman run --rm --privileged --net=host",             // coreos-installer fetches the metal image itself
		"-v /run/udev:/run/udev",                              // REQUIRED or coreos-installer fails (udevd socket missing)
		"coreos-installer:release",                            // the official container, not an apt binary
		`install "$LOOP" --ignition-file /data/devx-node.ign`, // install FCOS + Ignition onto the loop
		`losetup -d "$LOOP"`,                                  // release the loop; image is a bootable GPT disk
	} {
		if !strings.Contains(got, want) {
			t.Errorf("assembly script must contain %q", want)
		}
	}
	// No Ventoy / ISO-download / exFAT machinery survives.
	for _, gone := range []string{"Ventoy2Disk.sh", "ventoy.json", "mkfs.exfat", "DEVXDATA", "iso ignition embed", "jq -r"} {
		if strings.Contains(got, gone) {
			t.Errorf("assembly script must not contain Ventoy/ISO remnant %q", gone)
		}
	}
	// It must target a loop device, never a physical/host disk.
	if strings.Contains(got, "/dev/sda") || strings.Contains(got, "/dev/disk") {
		t.Error("assembly must not write to any physical/host disk")
	}
}

func TestRenderAssemblyScript_RequiresPaths(t *testing.T) {
	if _, err := RenderAssemblyScript(AssemblyParams{ImageVM: "/o.img"}); err == nil {
		t.Error("a ButaneVM path is required")
	}
	if _, err := RenderAssemblyScript(AssemblyParams{ButaneVM: "/i.bu"}); err == nil {
		t.Error("an ImageVM path is required")
	}
}

func TestRenderAssemblyScript_DefaultsImageSize(t *testing.T) {
	p := sampleAssembly()
	p.ImageSizeMB = 0
	got, err := RenderAssemblyScript(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, fmt.Sprintf("truncate -s %dM", DefaultImageSizeMB)) {
		t.Errorf("zero ImageSizeMB must fall back to DefaultImageSizeMB (%dM)", DefaultImageSizeMB)
	}
}
