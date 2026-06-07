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

// Runtime ISO resolution sources (x86_64; the feature targets x86 laptops). URLs
// are resolved at build time inside the VM so they never go stale: FCOS from its
// stream metadata, Ubuntu from the releases directory listing (point releases are
// pruned, so a pinned URL would 404).
const (
	FCOSStreamURL     = "https://builds.coreos.fedoraproject.org/streams/stable.json"
	UbuntuReleasesDir = "https://releases.ubuntu.com/24.04/"
	// CoreosInstallerImage embeds the Ignition; coreos-installer ships no Linux
	// binary on GitHub, so we run its official container in the builder VM.
	CoreosInstallerImage = "quay.io/coreos/coreos-installer:release"
)

// ISOSpec describes one OS image placed on the stick. Resolver selects how the
// download URL is obtained: "fcos" (stream metadata), "ubuntu" (releases dir),
// or "" (use URL verbatim).
type ISOSpec struct {
	Name          string // "fcos" | "ubuntu"
	URL           string
	Filename      string
	Resolver      string
	EmbedIgnition bool
}

// AssemblyParams is the fully-resolved input to RenderAssemblyScript.
type AssemblyParams struct {
	BootSizeMB  int
	TotalSizeMB int
	ISOs        []ISOSpec
	ButaneVM    string // path inside the VM to the FCOS Butane (.bu) to compile + embed
	PayloadVM   string // path inside the VM to staged ventoy.json + seeds
	ImageVM     string // path inside the VM for the output .img
}

// DefaultISOSpecs returns the built-in ephemeral ISO specs for the requested
// names (subset of "fcos", "ubuntu"). Filenames match the renderer BaseImage
// constants so the on-stick names line up with ventoy.json.
func DefaultISOSpecs(names []string) []ISOSpec {
	var out []ISOSpec
	for _, n := range names {
		switch strings.TrimSpace(n) {
		case "fcos":
			out = append(out, ISOSpec{Name: "fcos", Filename: FCOSImage, Resolver: "fcos", EmbedIgnition: true})
		case "ubuntu":
			out = append(out, ISOSpec{Name: "ubuntu", Filename: UbuntuImage, Resolver: "ubuntu"})
		}
	}
	return out
}

// RenderAssemblyScript returns the deterministic bash run inside the Lima VM to
// produce the full Ventoy disk image. Pure text (golden-tested); its execution
// against real Ventoy is validated by the gated integration test.
func RenderAssemblyScript(p AssemblyParams) (string, error) {
	var fcos *ISOSpec
	for i := range p.ISOs {
		if p.ISOs[i].EmbedIgnition {
			fcos = &p.ISOs[i]
		}
	}
	if fcos == nil {
		return "", fmt.Errorf("assembly requires one ISO with EmbedIgnition=true (FCOS)")
	}
	reserve := p.TotalSizeMB - p.BootSizeMB
	if reserve <= 0 {
		return "", fmt.Errorf("total size (%dM) must exceed boot size (%dM)", p.TotalSizeMB, p.BootSizeMB)
	}

	var b strings.Builder
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	w("#!/usr/bin/env bash\n# devx usb assembly — GENERATED.\nset -euxo pipefail\n\n")
	w("CACHE=/var/cache/devx-usb\nmkdir -p \"$CACHE\"\n\n")

	// --- Download ISOs (cached; URLs resolved at runtime where needed) ---
	for _, iso := range p.ISOs {
		switch iso.Resolver {
		case "fcos":
			w("if [ ! -f \"$CACHE/%s\" ]; then\n", iso.Filename)
			w("  url=$(wget -qO- %q | jq -r '.architectures.x86_64.artifacts.metal.formats.iso.disk.location')\n", FCOSStreamURL)
			w("  wget -q -O \"$CACHE/%s.part\" \"$url\" && mv \"$CACHE/%s.part\" \"$CACHE/%s\"\nfi\n", iso.Filename, iso.Filename, iso.Filename)
		case "ubuntu":
			w("if [ ! -f \"$CACHE/%s\" ]; then\n", iso.Filename)
			w("  name=$(wget -qO- %q | grep -oE 'ubuntu-24[.]04[0-9.]*-live-server-amd64[.]iso' | sort -u | tail -n1)\n", UbuntuReleasesDir)
			w("  wget -q -O \"$CACHE/%s.part\" %q\"$name\" && mv \"$CACHE/%s.part\" \"$CACHE/%s\"\nfi\n", iso.Filename, UbuntuReleasesDir, iso.Filename, iso.Filename)
		default:
			w("if [ ! -f \"$CACHE/%s\" ]; then wget -q -O \"$CACHE/%s.part\" %q && mv \"$CACHE/%s.part\" \"$CACHE/%s\"; fi\n",
				iso.Filename, iso.Filename, iso.URL, iso.Filename, iso.Filename)
		}
	}

	// Work dir (scratch + image) sits next to the image, on the persistent root
	// disk (NOT tmpfs) so the multi-GB sparse image + ISO copy have room.
	w("\nIMG=%q\nWORK=$(dirname \"$IMG\")\nmkdir -p \"$WORK\"\n\n", p.ImageVM)

	// --- Compile Butane → Ignition and embed into a copy of the FCOS ISO ---
	w("butane --strict -o \"$WORK/devx-node.ign\" %q\n", p.ButaneVM)
	w("cp \"$CACHE/%s\" \"$WORK/fcos-devx.iso\"\n", fcos.Filename)
	w("podman run --rm --privileged -v \"$WORK\":/data %s iso ignition embed -f -i /data/devx-node.ign /data/fcos-devx.iso\n\n", CoreosInstallerImage)

	// --- Sparse image + loop + Ventoy install (reserve the storage tail) ---
	w("rm -f \"$IMG\"\ntruncate -s %dM \"$IMG\"\nLOOP=$(losetup --show -f -P \"$IMG\")\n", p.TotalSizeMB)
	b.WriteString("trap 'umount /mnt/vtoy 2>/dev/null || true; [ -n \"${LOOP:-}\" ] && losetup -d \"$LOOP\" 2>/dev/null || true' EXIT\n")
	// Confirmations via heredoc, not `yes |` — a pipe makes `yes` take SIGPIPE
	// when Ventoy stops reading, which fails the pipeline under `set -o pipefail`
	// even on a successful install.
	w("bash /opt/ventoy/Ventoy2Disk.sh -I -r %d \"$LOOP\" <<'VTEOF'\ny\ny\nVTEOF\n\n", reserve)

	// --- Storage partition: fill Ventoy's reserved tail, derived at runtime ---
	b.WriteString(`# Start = end of Ventoy's last partition (so the exFAT storage exactly fills the
# reserved tail). DEVXDATA is general-purpose storage, intentionally left empty.
udevadm settle 2>/dev/null || true
start=$(parted -ms "$LOOP" unit MiB print | tail -n1 | awk -F: '{gsub(/MiB/,"",$3); print $3}')
parted -s "$LOOP" -- unit MiB mkpart primary "${start}" 100%
partx -u "$LOOP" 2>/dev/null || partprobe "$LOOP" 2>/dev/null || true
udevadm settle 2>/dev/null || true
for i in $(seq 1 30); do [ -b "${LOOP}p3" ] && break; sleep 0.5; done
mkfs.exfat -L DEVXDATA "${LOOP}p3"

`)

	// --- Copy ISOs + payloads onto the Ventoy data partition (p1) ---
	w("mkdir -p /mnt/vtoy\nmount \"${LOOP}p1\" /mnt/vtoy\n")
	w("cp \"$WORK/fcos-devx.iso\" /mnt/vtoy/%s\n", fcos.Filename)
	for _, iso := range p.ISOs {
		if iso.EmbedIgnition {
			continue
		}
		w("cp \"$CACHE/%s\" /mnt/vtoy/%s\n", iso.Filename, iso.Filename)
	}
	w("mkdir -p /mnt/vtoy/ventoy\ncp %q/ventoy/ventoy.json /mnt/vtoy/ventoy/ventoy.json\n", p.PayloadVM)
	// Ubuntu cloud-init seed (referenced by ventoy.json auto_install as /ubuntu/.../user-data).
	w("if [ -d %q/ubuntu ]; then cp -r %q/ubuntu /mnt/vtoy/ubuntu; fi\n", p.PayloadVM, p.PayloadVM)
	w("rm -f \"$WORK/fcos-devx.iso\" \"$WORK/devx-node.ign\"\n") // free ~1GB of scratch
	b.WriteString("sync\numount /mnt/vtoy\necho \"devx: image ready at $IMG\"\n")
	return b.String(), nil
}
