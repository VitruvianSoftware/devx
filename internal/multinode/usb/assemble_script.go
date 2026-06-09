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
	"strings"
)

const (
	// CoreosInstallerImage runs the install. coreos-installer ships no Linux
	// binary on GitHub, so we run its official container in the builder VM. It
	// fetches the Fedora CoreOS metal image from the stream itself, so there is
	// no ISO download and nothing for devx to vendor.
	CoreosInstallerImage = "quay.io/coreos/coreos-installer:release"

	// DefaultImageSizeMB sizes the sparse image just for the Fedora CoreOS metal
	// layout (BIOS-BOOT, EFI-SYSTEM, boot, root). FCOS grows root to fill the real
	// USB on first boot, so a few GB is plenty here.
	DefaultImageSizeMB = 4096
)

// AssemblyParams is the fully-resolved input to RenderAssemblyScript.
type AssemblyParams struct {
	// ImageSizeMB sizes the sparse image. Zero falls back to DefaultImageSizeMB.
	ImageSizeMB int
	ButaneVM    string // path inside the VM to the FCOS Butane (.bu) to compile
	ImageVM     string // path inside the VM for the output GPT .img
}

// RenderAssemblyScript returns the deterministic bash run inside the Lima VM to
// produce a bootable GPT Fedora CoreOS image. It compiles the join Butane to
// Ignition, then runs coreos-installer (in its official container) to install
// the FCOS metal image + Ignition onto a loop-mounted sparse image. The
// resulting image flashes to a USB on macOS where coreos-installer cannot run.
// Pure text (golden-tested); execution is validated by the gated integration
// test.
func RenderAssemblyScript(p AssemblyParams) (string, error) {
	if p.ButaneVM == "" {
		return "", fmt.Errorf("assembly requires a ButaneVM path to compile")
	}
	if p.ImageVM == "" {
		return "", fmt.Errorf("assembly requires an ImageVM output path")
	}
	sizeMB := p.ImageSizeMB
	if sizeMB <= 0 {
		sizeMB = DefaultImageSizeMB
	}

	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	w("#!/usr/bin/env bash\n# devx usb assembly — GENERATED.\nset -euxo pipefail\n\n")

	// Work dir (Ignition + sparse image) sits next to the image, on the persistent
	// root disk (NOT tmpfs) so the multi-GB image has room.
	w("IMG=%q\nWORK=$(dirname \"$IMG\")\nmkdir -p \"$WORK\"\n\n", p.ImageVM)

	// --- Compile the join Butane → Ignition ---
	w("butane --strict -o \"$WORK/devx-node.ign\" %q\n\n", p.ButaneVM)

	// --- Sparse image + loop device ---
	w("rm -f \"$IMG\"\ntruncate -s %dM \"$IMG\"\nLOOP=$(losetup --show -f -P \"$IMG\")\n", sizeMB)
	b.WriteString("trap '[ -n \"${LOOP:-}\" ] && losetup -d \"$LOOP\" 2>/dev/null || true' EXIT\n\n")

	// --- coreos-installer install: FCOS metal image + Ignition onto the loop ---
	// The /run/udev mount is REQUIRED — without it coreos-installer fails with
	// "udevd socket missing". --net=host lets it fetch the metal image from the
	// FCOS stream. The result is a GPT disk (BIOS-BOOT, EFI-SYSTEM, boot, root);
	// FCOS grows root to fill the real USB on first boot.
	w("podman run --rm --privileged --net=host --security-opt label=disable \\\n")
	w("  -v /dev:/dev -v /run/udev:/run/udev -v \"$WORK\":/data \\\n")
	w("  %s \\\n", CoreosInstallerImage)
	w("  install \"$LOOP\" --ignition-file /data/devx-node.ign\n\n")

	// --- Release the loop; the image is a bootable GPT FCOS disk ---
	b.WriteString("losetup -d \"$LOOP\"\nLOOP=\"\"\n")
	b.WriteString("rm -f \"$WORK/devx-node.ign\"\n")
	b.WriteString("sync\necho \"devx: image ready at $IMG\"\n")
	return b.String(), nil
}
