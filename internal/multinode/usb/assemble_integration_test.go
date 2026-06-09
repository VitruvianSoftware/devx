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

//go:build integration

// Run with: go test -tags integration ./internal/multinode/usb/ -run Integration -timeout 30m
//
// This builds a real GPT Fedora CoreOS image in a Lima VM (coreos-installer
// fetches the FCOS metal image from the stream, embeds the compiled Ignition,
// and installs it to a loop-mounted sparse image) and inspects the result inside
// the VM. It is heavy (GBs of download, minutes of work) and gated behind the
// `integration` build tag + limactl presence, so it never runs in the normal
// unit suite.
package usb

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestAssembleIntegration(t *testing.T) {
	if _, err := exec.LookPath("limactl"); err != nil {
		t.Skip("limactl not present")
	}
	ctx := context.Background()

	// Stage a minimal FCOS-only payload (no cluster needed): render the FCOS
	// Butane into a temp dir. coreos-installer compiles + embeds it.
	staging := t.TempDir()
	sink := DirSink{Root: staging}
	spec := JoinSpec{
		Role: RoleAgent, Mode: ModeEphemeral, Pool: "usb", NodeNamePrefix: "itest",
		Token: "itesttoken", LANServerURL: "https://10.0.0.5:6443", Ephemeral: true,
		Scratch: ScratchConfig{Strategy: ScratchNone},
	}
	if _, err := (FCOSRenderer{}).Render(spec, ModeEphemeral, sink); err != nil {
		t.Fatal(err)
	}

	b := NewBuilder("devx-usb-itest")
	if err := b.EnsureBuilderVM(ctx); err != nil {
		t.Fatalf("ensure builder VM: %v", err)
	}

	// Size only for the FCOS metal layout; FCOS grows root on first boot.
	params := AssemblyParams{
		ImageSizeMB: 4096,
		ButaneVM:    "/var/tmp/devx/payload/fcos/ephemeral/devx-join.bu",
		ImageVM:     "/var/tmp/devx/devx-usb.img",
	}
	img, err := b.BuildImage(ctx, params, staging)
	if err != nil {
		t.Fatalf("build image: %v", err)
	}
	if fi, err := os.Stat(img); err != nil || fi.Size() < 1<<30 {
		t.Fatalf("image %s missing or too small: %v", img, err)
	}

	// BuildImage reclaims the image inside the VM, so copy the produced image back
	// in to inspect its Linux partitions (macOS can't read them).
	const inspectImg = "/var/tmp/inspect.img"
	if out, err := exec.CommandContext(ctx, "limactl", "copy", img, "devx-usb-itest:"+inspectImg).CombinedOutput(); err != nil {
		t.Fatalf("copy image into VM for inspection: %v\n%s", err, out)
	}

	out, err := exec.CommandContext(ctx, "limactl", "shell", "devx-usb-itest", "sudo", "bash", "-c",
		"L=$(losetup --show -f -P "+inspectImg+"); sgdisk -p $L 2>/dev/null || parted -s $L print; blkid; losetup -d $L").CombinedOutput()
	if err != nil {
		t.Fatalf("inspect image: %v\n%s", err, out)
	}
	got := string(out)
	// The coreos-installer GPT layout: BIOS-BOOT, EFI-SYSTEM (vfat), boot (ext4),
	// root (xfs).
	for _, want := range []string{"EFI-SYSTEM", "boot", "root"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected GPT partition %q in:\n%s", want, got)
		}
	}
	// The Ventoy-era exFAT storage partition must be gone.
	if strings.Contains(got, "DEVXDATA") {
		t.Errorf("unexpected exFAT DEVXDATA storage partition (Ventoy remnant):\n%s", got)
	}
}
