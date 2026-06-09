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
	"io/fs"
	"sort"
)

// InjectionKind identifies how a boot entry's provisioning payload attaches to
// its boot medium.
type InjectionKind string

const (
	// InjectIgnition delivers a Fedora CoreOS Ignition (compiled from Butane).
	InjectIgnition InjectionKind = "ignition"
	// InjectCloudInit delivers a cloud-init NoCloud/autoinstall seed.
	InjectCloudInit InjectionKind = "cloud-init"
	// InjectConfigPartition delivers a small FAT partition image a prebaked OS
	// reads at boot.
	InjectConfigPartition InjectionKind = "config-partition"
)

// Artifact is a single file a renderer wants written under the staging root.
type Artifact struct {
	Path     string      // relative path under the output root (forward slashes)
	Contents []byte      // file contents
	Mode     fs.FileMode // 0 means 0644
}

// Injection describes the provisioning payload attached to one boot entry.
type Injection struct {
	Kind        InjectionKind
	PayloadPath string // relative path of the payload artifact in the sink
	Notes       string // operator-facing notes
}

// BootEntry is one boot option plus the metadata the assembler needs to wire its
// provisioning payload.
type BootEntry struct {
	MenuTitle     string    // human label for the boot entry
	BaseImage     string    // boot medium this entry produces/boots
	Renderer      string    // provenance: renderer Name()
	Mode          Mode      // provenance: lifecycle mode
	InstallToDisk bool      // true if this entry persists the OS to the internal disk
	Injection     Injection // how the join payload attaches
}

// ArtifactSink receives the files a renderer produces.
type ArtifactSink interface {
	Add(Artifact) error
}

// Renderer turns a JoinSpec into boot artifacts for one OS/packaging. Renderers
// are pure: they only produce in-memory artifacts and never shell out, so they
// are fully unit-testable. External tooling (butane compile, ISO embedding, USB
// writes) lives in the build/assemble layer and is gated on tool presence.
type Renderer interface {
	Name() string
	Modes() []Mode
	Render(spec JoinSpec, m Mode, sink ArtifactSink) ([]BootEntry, error)
}

// AllRenderers returns the built-in renderers keyed by Name(), in a stable order.
func AllRenderers() []Renderer {
	return []Renderer{FCOSRenderer{}, BakedRenderer{}}
}

// RendererByName looks up a built-in renderer.
func RendererByName(name string) (Renderer, error) {
	for _, r := range AllRenderers() {
		if r.Name() == name {
			return r, nil
		}
	}
	return nil, fmt.Errorf("unknown renderer %q (have %v)", name, RendererNames())
}

// RendererNames returns the built-in renderer names, sorted.
func RendererNames() []string {
	var names []string
	for _, r := range AllRenderers() {
		names = append(names, r.Name())
	}
	sort.Strings(names)
	return names
}

// modeSupported reports whether r advertises support for mode m.
func modeSupported(r Renderer, m Mode) bool {
	for _, sm := range r.Modes() {
		if sm == m {
			return true
		}
	}
	return false
}

// menuTitle builds a consistent boot-entry label.
func menuTitle(os string, m Mode) string {
	switch m {
	case ModeInstall:
		return fmt.Sprintf("Install node to disk — %s", os)
	default:
		return fmt.Sprintf("Join cluster (ephemeral) — %s", os)
	}
}
