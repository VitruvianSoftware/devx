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

// Mount handling for the lima package: resolving the configured mount set,
// rendering it into lima.yaml, parsing it back, and producing a script to
// reconcile it on an existing VM.
package lima

import (
	"fmt"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"gopkg.in/yaml.v3"
)

// DefaultMountType is the Lima file-sharing backend used for vmType "vz".
const DefaultMountType = "virtiofs"

// ResolveMounts returns the effective mount set for the cluster:
//
//	nil   (mounts: key omitted) -> default: one writable mount of the host home dir ("~")
//	[]    (mounts: [])          -> opt out: no mounts
//	[...] (explicit entries)    -> used as-is
//
// "~" is intentional: Lima expands it on the target host and mounts it at the
// same absolute path inside the guest, which is what makes Docker bind mounts of
// host paths resolve inside the VM.
func ResolveMounts(m []config.MountConfig) []config.MountConfig {
	if m == nil {
		return []config.MountConfig{{Location: "~", Writable: true}}
	}
	return m
}

// renderMounts renders the `mountType:` + `mounts:` block for a lima.yaml.
// Returns "" when there are no mounts (so an opt-out emits nothing).
func renderMounts(mounts []config.MountConfig) string {
	if len(mounts) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "mountType: %q\n", DefaultMountType)
	b.WriteString("mounts:\n")
	for _, m := range mounts {
		fmt.Fprintf(&b, "  - location: %q\n", m.Location)
		if m.MountPoint != "" && m.MountPoint != m.Location {
			fmt.Fprintf(&b, "    mountPoint: %q\n", m.MountPoint)
		}
		if m.Writable {
			b.WriteString("    writable: true\n")
		}
	}
	return b.String()
}

// limaSpecDoc is the subset of lima.yaml devx manages and reconciles.
type limaSpecDoc struct {
	CPUs      int              `yaml:"cpus"`
	Memory    string           `yaml:"memory"`
	Disk      string           `yaml:"disk"`
	MountType string           `yaml:"mountType"`
	Mounts    []limaMountEntry `yaml:"mounts"`
}

type limaMountEntry struct {
	Location   string `yaml:"location"`
	MountPoint string `yaml:"mountPoint"`
	Writable   bool   `yaml:"writable"`
}

// Spec is the parsed, devx-managed subset of a live lima.yaml.
type Spec struct {
	CPUs   int
	Memory string
	Disk   string
	Mounts []config.MountConfig
}

// ParseSpec extracts the devx-managed fields from a lima.yaml document.
func ParseSpec(limaYAML string) (Spec, error) {
	var doc limaSpecDoc
	if err := yaml.Unmarshal([]byte(limaYAML), &doc); err != nil {
		return Spec{}, fmt.Errorf("parsing lima.yaml: %w", err)
	}
	mounts := make([]config.MountConfig, 0, len(doc.Mounts))
	for _, m := range doc.Mounts {
		mounts = append(mounts, config.MountConfig{Location: m.Location, MountPoint: m.MountPoint, Writable: m.Writable})
	}
	return Spec{CPUs: doc.CPUs, Memory: doc.Memory, Disk: doc.Disk, Mounts: mounts}, nil
}

// MountsEqual reports whether two mount sets are equivalent. It is order-
// sensitive and normalizes an empty MountPoint to Location (since Lima treats an
// omitted mountPoint as equal to location).
func MountsEqual(a, b []config.MountConfig) bool {
	if len(a) != len(b) {
		return false
	}
	norm := func(m config.MountConfig) config.MountConfig {
		if m.MountPoint == "" {
			m.MountPoint = m.Location
		}
		return m
	}
	for i := range a {
		if norm(a[i]) != norm(b[i]) {
			return false
		}
	}
	return true
}

// MountsRewriteScript returns a bash script that replaces the `mountType:` line
// and `mounts:` block in ~/.lima/<vmName>/lima.yaml with the rendered desired
// set, leaving all other keys untouched. It is base64-piped to the host to avoid
// quoting issues (see Provision). An empty mount set strips the block entirely.
// The mount type is always (re)set to DefaultMountType.
func MountsRewriteScript(vmName string, mounts []config.MountConfig) string {
	block := renderMounts(mounts)
	// awk drops the existing mountType: line and the entire mounts: block (the
	// `mounts:` line plus its indented list items), ending the block at the next
	// top-level mapping key (a letter in column 0).
	return fmt.Sprintf(`set -e
cd ~/.lima/%s
awk '
/^mounts:/ { inblock=1; next }
inblock && /^[a-zA-Z]/ { inblock=0 }
inblock { next }
/^mountType:/ { next }
{ print }
' lima.yaml > lima.yaml.devx.tmp
cat >> lima.yaml.devx.tmp <<'DEVXEOF'
%sDEVXEOF
mv lima.yaml.devx.tmp lima.yaml
`, vmName, block)
}
