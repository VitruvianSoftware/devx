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
// This builds a real Ventoy image in a Lima VM (downloads the FCOS live ISO,
// embeds an Ignition, installs Ventoy, formats an exFAT storage partition) and
// inspects the result inside the VM. It is heavy (GBs of download, minutes of
// work) and gated behind the `integration` build tag + limactl presence, so it
// never runs in the normal unit suite.
package usb

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssembleIntegration(t *testing.T) {
	if _, err := exec.LookPath("limactl"); err != nil {
		t.Skip("limactl not present")
	}
	ctx := context.Background()

	// Stage a minimal FCOS-only payload (no cluster needed): render the FCOS
	// Butane + a ventoy.json into a temp dir.
	staging := t.TempDir()
	sink := DirSink{Root: staging}
	spec := JoinSpec{
		Role: RoleAgent, Mode: ModeEphemeral, Pool: "usb", NodeNamePrefix: "itest",
		Token: "itesttoken", LANServerURL: "https://10.0.0.5:6443", Ephemeral: true,
		Scratch: ScratchConfig{Strategy: ScratchNone},
	}
	entries, err := FCOSRenderer{}.Render(spec, ModeEphemeral, sink)
	if err != nil {
		t.Fatal(err)
	}
	vj, err := VentoyJSON(entries)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Add(Artifact{Path: "ventoy/ventoy.json", Contents: vj}); err != nil {
		t.Fatal(err)
	}

	b := NewBuilder("devx-usb-itest")
	if err := b.EnsureBuilderVM(ctx); err != nil {
		t.Fatalf("ensure builder VM: %v", err)
	}

	// FCOS-only, small image: 3 GB boot (FCOS live ISO ~1 GB) + 2 GB exFAT.
	params := AssemblyParams{
		BootSizeMB: 3072, TotalSizeMB: 5120,
		ISOs:      DefaultISOSpecs([]string{"fcos"}),
		ButaneVM:  "/var/tmp/devx/payload/fcos/ephemeral/devx-join.bu",
		PayloadVM: "/var/tmp/devx/payload",
		ImageVM:   "/var/tmp/devx/devx-usb.img",
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
		"L=$(losetup --show -f -P "+inspectImg+"); parted -s $L print; blkid; losetup -d $L").CombinedOutput()
	if err != nil {
		t.Fatalf("inspect image: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "DEVXDATA") {
		t.Errorf("expected an exFAT DEVXDATA storage partition; got:\n%s", got)
	}

	// Confirm the embedded Ignition is readable from the FCOS ISO on the boot
	// partition. coreos-installer ships no binary, so read it via the container.
	isoName := filepath.Base(DefaultISOSpecs([]string{"fcos"})[0].Filename)
	ign, err := exec.CommandContext(ctx, "limactl", "shell", "devx-usb-itest", "sudo", "bash", "-c",
		"set -e; L=$(losetup --show -f -P "+inspectImg+"); mkdir -p /mnt/i; mount ${L}p1 /mnt/i; "+
			"podman run --rm -v /mnt/i:/data "+CoreosInstallerImage+" iso ignition show /data/"+isoName+"; "+
			"umount /mnt/i; losetup -d $L").CombinedOutput()
	if err != nil || !strings.Contains(string(ign), "devx-join") {
		t.Errorf("embedded Ignition not found on FCOS ISO: %v\n%s", err, ign)
	}
}
