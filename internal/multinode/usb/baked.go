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

import "fmt"

// BakedImage is the prebuilt, self-contained node image the baked entries boot.
// It already contains k3s + Tailscale + a generic devx-join unit; only the
// per-cluster secrets (the join script) live on the separate config partition,
// so the image itself carries no credentials. Built by CI / a future
// `devx cluster usb bake`.
const BakedImage = "devx-node.x86_64.img"

// BakedRenderer separates a prebuilt OS image (with k3s+Tailscale baked in) from
// the per-cluster secrets, which it emits onto a small FAT config partition the
// image reads at boot. Fastest/most offline-robust boot at the cost of a build
// pipeline for the image.
type BakedRenderer struct{}

// Name implements Renderer.
func (BakedRenderer) Name() string { return "baked" }

// Modes implements Renderer.
func (BakedRenderer) Modes() []Mode { return []Mode{ModeEphemeral, ModeInstall} }

// Render implements Renderer. It emits the config-partition payload: the join
// script and a mode flag the baked image's generic unit reads at boot. No
// credentials are in the image — only here.
func (BakedRenderer) Render(spec JoinSpec, m Mode, sink ArtifactSink) ([]BootEntry, error) {
	if !modeSupported(BakedRenderer{}, m) {
		return nil, fmt.Errorf("baked: unsupported mode %q", m)
	}
	spec.Mode = m

	script, err := RenderJoinScript(spec)
	if err != nil {
		return nil, fmt.Errorf("baked: rendering join script: %w", err)
	}

	dir := fmt.Sprintf("baked/%s/devx-config", m)
	readme := "# devx node config partition\n" +
		"# Write these files to a FAT partition labeled DEVXCFG. The prebaked\n" +
		"# image's devx-join unit mounts it, runs devx-join.sh, and reads mode.\n"

	for _, a := range []Artifact{
		{Path: dir + "/devx-join.sh", Contents: []byte(script), Mode: 0o755},
		{Path: dir + "/mode", Contents: []byte(string(m) + "\n"), Mode: 0o644},
		{Path: dir + "/README", Contents: []byte(readme), Mode: 0o644},
	} {
		if err := sink.Add(a); err != nil {
			return nil, err
		}
	}

	return []BootEntry{{
		MenuTitle:     menuTitle("devx prebaked image", m),
		BaseImage:     BakedImage,
		Renderer:      "baked",
		Mode:          m,
		InstallToDisk: m == ModeInstall,
		Injection: Injection{
			Kind:        InjectConfigPartition,
			PayloadPath: dir,
			Notes:       "Write payload to a FAT partition labeled DEVXCFG; the baked image reads it at boot.",
		},
	}}, nil
}
