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

// Default ephemeral ISO sources (x86_64; the feature targets x86 laptops). The
// FCOS live ISO URL is resolved at runtime from the stable-stream metadata so it
// does not go stale; Ubuntu's is pinned to an LTS point release.
const (
	UbuntuISOURL  = "https://releases.ubuntu.com/24.04/ubuntu-24.04.1-live-server-amd64.iso"
	FCOSStreamURL = "https://builds.coreos.fedoraproject.org/streams/stable.json"
)

// ISOSpec describes one OS image placed on the stick. An empty URL combined with
// EmbedIgnition=true means "resolve the FCOS live ISO from the stream metadata."
type ISOSpec struct {
	Name          string // "fcos" | "ubuntu"
	URL           string
	Filename      string
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
// names (subset of "fcos", "ubuntu").
func DefaultISOSpecs(names []string) []ISOSpec {
	var out []ISOSpec
	for _, n := range names {
		switch strings.TrimSpace(n) {
		case "fcos":
			out = append(out, ISOSpec{Name: "fcos", Filename: "fedora-coreos-live.x86_64.iso", EmbedIgnition: true})
		case "ubuntu":
			out = append(out, ISOSpec{Name: "ubuntu", URL: UbuntuISOURL, Filename: "ubuntu-24.04-live-server-amd64.iso"})
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
	for _, iso := range p.ISOs {
		if iso.URL == "" && iso.EmbedIgnition {
			// FCOS: resolve the current live ISO URL from the stream metadata.
			w("if [ ! -f \"$CACHE/%s\" ]; then\n", iso.Filename)
			w("  url=$(wget -qO- %q | jq -r '.architectures.x86_64.artifacts.metal.formats.iso.disk.location')\n", FCOSStreamURL)
			w("  wget -q -O \"$CACHE/%s.part\" \"$url\" && mv \"$CACHE/%s.part\" \"$CACHE/%s\"\nfi\n", iso.Filename, iso.Filename, iso.Filename)
		} else {
			w("if [ ! -f \"$CACHE/%s\" ]; then wget -q -O \"$CACHE/%s.part\" %q && mv \"$CACHE/%s.part\" \"$CACHE/%s\"; fi\n",
				iso.Filename, iso.Filename, iso.URL, iso.Filename, iso.Filename)
		}
	}
	w("\n# Compile Butane → Ignition, then embed into a copy of the FCOS ISO\n")
	w("butane --strict -o /tmp/devx-node.ign %q\n", p.ButaneVM)
	w("cp \"$CACHE/%s\" /tmp/fcos-devx.iso\n", fcos.Filename)
	b.WriteString("coreos-installer iso ignition embed -f -i /tmp/devx-node.ign /tmp/fcos-devx.iso\n\n")
	w("# Sparse image + loop\nrm -f %q\ntruncate -s %dM %q\nLOOP=$(losetup --show -f -P %q)\n\n", p.ImageVM, p.TotalSizeMB, p.ImageVM, p.ImageVM)
	w("# Install Ventoy (force, reserve the storage tail)\nyes | bash /opt/ventoy/Ventoy2Disk.sh -I -r %d \"$LOOP\"\npartprobe \"$LOOP\" || true\nsleep 2\n\n", reserve)
	w("# Storage partition in the reserved tail → exFAT (3rd primary in free space)\n")
	w("parted -s \"$LOOP\" -- mkpart primary 3 -1s\npartprobe \"$LOOP\" || true\nsleep 2\nmkfs.exfat -n DEVXDATA \"${LOOP}p3\"\n\n")
	w("# Copy ISOs + payloads onto the Ventoy data partition\nmkdir -p /mnt/vtoy\nmount \"${LOOP}p1\" /mnt/vtoy\n")
	w("cp /tmp/fcos-devx.iso /mnt/vtoy/%s\n", fcos.Filename)
	for _, iso := range p.ISOs {
		if iso.EmbedIgnition {
			continue
		}
		w("cp \"$CACHE/%s\" /mnt/vtoy/%s\n", iso.Filename, iso.Filename)
	}
	w("mkdir -p /mnt/vtoy/ventoy\ncp %q/ventoy/ventoy.json /mnt/vtoy/ventoy/ventoy.json\n", p.PayloadVM)
	// Ubuntu cloud-init seed (referenced by ventoy.json auto_install as /ubuntu/.../user-data).
	w("cp -r %q/ubuntu /mnt/vtoy/ubuntu 2>/dev/null || true\n", p.PayloadVM)
	w("sync\numount /mnt/vtoy\nlosetup -d \"$LOOP\"\necho \"devx: image ready at %s\"\n", p.ImageVM)
	return b.String(), nil
}
