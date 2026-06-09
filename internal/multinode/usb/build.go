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
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

// BuildOptions configures a `devx cluster usb build`.
type BuildOptions struct {
	OutputDir string   // staging directory for artifacts (required unless DryRun)
	Renderers []string // subset of fcos|baked; empty uses config/all
	Modes     []Mode   // subset of ephemeral|install; empty uses both
	Role      Role     // role for install entries (ephemeral always joins as agent)
	DryRun    bool     // plan only; do not resolve coordinates or write files
}

// PlanEntry is one boot-menu entry in a build result (JSON-friendly).
type PlanEntry struct {
	Renderer  string `json:"renderer"`
	Mode      Mode   `json:"mode"`
	MenuTitle string `json:"menu_title"`
	BaseImage string `json:"base_image"`
	Injection string `json:"injection"`
}

// BuildResult summarizes a build for human and --json output.
type BuildResult struct {
	OutputDir      string      `json:"output_dir,omitempty"`
	DryRun         bool        `json:"dry_run"`
	Entries        []PlanEntry `json:"entries"`
	RequiredImages []string    `json:"required_images"`
}

// Build resolves the join coordinates, runs the selected renderers across the
// selected modes into OutputDir, and writes an operator manifest. With DryRun it
// produces only the plan (no cluster access, no files).
func Build(ctx context.Context, cfg *config.Config, opts BuildOptions) (*BuildResult, error) {
	renderers, err := selectRenderers(opts.Renderers, cfg.Cluster.USB.Renderers)
	if err != nil {
		return nil, err
	}
	modes := opts.Modes
	if len(modes) == 0 {
		modes = []Mode{ModeEphemeral, ModeInstall}
	}

	scratch, err := parseScratch(cfg.Cluster.USB.Scratch)
	if err != nil {
		return nil, err
	}

	base := JoinSpec{
		K3sVersion:       cfg.Cluster.K3sVersion,
		Pool:             orDefault(cfg.Cluster.USB.Pool, "usb"),
		NodeNamePrefix:   orDefault(cfg.Cluster.USB.NodeNamePrefix, "usb"),
		WiFiSSID:         cfg.Cluster.USB.WiFi.SSID,
		WiFiPSK:          cfg.Cluster.USB.WiFi.PSK,
		TailscaleVersion: DefaultTailscaleVersion,
		Ephemeral:        true, // overridden per-mode below
		Scratch:          scratch,
	}

	if opts.DryRun {
		base.LANServerURL = "https://CLUSTER-API:6443"
		base.Token = "DRY-RUN-TOKEN"
	} else {
		coords, err := ResolveCoordinates(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("resolving cluster coordinates: %w", err)
		}
		base.LANServerURL = coords.LANServerURL
		base.TailnetServerURL = coords.TailnetServerURL
		base.Token = coords.Token
		if coords.K3sVersion != "" {
			base.K3sVersion = coords.K3sVersion
		}
		// A tailnet URL needs a Tailscale auth key baked in to be reachable.
		if base.TailnetServerURL != "" {
			base.TailscaleAuthKey = cfg.Cluster.Tailscale.AuthKey
			if base.TailscaleAuthKey == "" {
				// Can't use the tailnet endpoint without a key; drop it.
				base.TailnetServerURL = ""
			}
		}
	}

	var sink ArtifactSink = discardSink{}
	if !opts.DryRun {
		if opts.OutputDir == "" {
			return nil, fmt.Errorf("OutputDir is required unless DryRun")
		}
		sink = DirSink{Root: opts.OutputDir}
	}

	var entries []BootEntry
	for _, r := range renderers {
		for _, m := range modes {
			if !modeSupported(r, m) {
				continue
			}
			spec := base
			spec.Mode = m
			if m == ModeEphemeral {
				spec.Role = RoleAgent // ephemeral nodes must never be etcd voters
				spec.Ephemeral = true
			} else {
				spec.Role = orRole(opts.Role, RoleAgent)
				spec.Ephemeral = false
				spec.Scratch = ScratchConfig{Strategy: ScratchNone} // install uses the whole disk
			}
			es, err := r.Render(spec, m, sink)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", r.Name(), m, err)
			}
			entries = append(entries, es...)
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no boot entries produced (renderers=%v modes=%v)", opts.Renderers, modes)
	}

	result := newBuildResult(entries, opts)

	if !opts.DryRun {
		if err := sink.Add(Artifact{Path: "MANIFEST.md", Contents: []byte(manifest(result)), Mode: 0o644}); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func newBuildResult(entries []BootEntry, opts BuildOptions) *BuildResult {
	r := &BuildResult{DryRun: opts.DryRun, OutputDir: opts.OutputDir}
	images := map[string]struct{}{}
	for _, e := range entries {
		r.Entries = append(r.Entries, PlanEntry{
			Renderer:  e.Renderer,
			Mode:      e.Mode,
			MenuTitle: e.MenuTitle,
			BaseImage: e.BaseImage,
			Injection: string(e.Injection.Kind),
		})
		if e.BaseImage != "" {
			images[e.BaseImage] = struct{}{}
		}
	}
	for img := range images {
		r.RequiredImages = append(r.RequiredImages, img)
	}
	sort.Strings(r.RequiredImages)
	return r
}

// selectRenderers resolves the renderer list: explicit flag, then config, then
// all built-ins.
func selectRenderers(flagList, cfgList []string) ([]Renderer, error) {
	names := flagList
	if len(names) == 0 {
		names = cfgList
	}
	if len(names) == 0 {
		return AllRenderers(), nil
	}
	var out []Renderer
	for _, n := range names {
		r, err := RendererByName(strings.TrimSpace(n))
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func parseScratch(s string) (ScratchConfig, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || s == string(ScratchNone):
		return ScratchConfig{Strategy: ScratchNone}, nil
	case s == string(ScratchFreeSpace):
		return ScratchConfig{Strategy: ScratchFreeSpace}, nil
	case strings.HasPrefix(s, "existing:"):
		src := strings.TrimPrefix(s, "existing:")
		if src == "" {
			return ScratchConfig{}, fmt.Errorf("scratch 'existing:' requires a source (LABEL=, UUID= or /dev/...)")
		}
		return ScratchConfig{Strategy: ScratchExisting, Source: src}, nil
	default:
		return ScratchConfig{}, fmt.Errorf("invalid scratch %q (want none|free-space|existing:<src>)", s)
	}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func orRole(v, def Role) Role {
	if v == "" {
		return def
	}
	return v
}

// discardSink drops artifacts; used for dry-run planning.
type discardSink struct{}

func (discardSink) Add(Artifact) error { return nil }

// manifest renders an operator-facing summary of the build.
func manifest(r *BuildResult) string {
	var b strings.Builder
	b.WriteString("# devx cluster USB — build manifest\n\n")
	b.WriteString("Boot entries:\n\n")
	for _, e := range r.Entries {
		fmt.Fprintf(&b, "- %s  (%s/%s, %s)\n", e.MenuTitle, e.Renderer, e.Mode, e.Injection)
	}
	b.WriteString("\nBoot media:\n\n")
	for _, img := range r.RequiredImages {
		fmt.Fprintf(&b, "- %s\n", img)
	}
	b.WriteString("\nThe provisioning payloads (Ignition/cloud-init/config) are staged alongside\n")
	b.WriteString("this manifest. The FCOS entry is delivered as a native GPT Fedora CoreOS disk\n")
	b.WriteString("built by coreos-installer (the metal image is fetched from the FCOS stream and\n")
	b.WriteString("the Ignition is embedded), not a multi-boot stick. To build a flashable image,\n")
	b.WriteString("re-run with --assemble (writes a .img in a Lima VM) or --device /dev/diskN\n")
	b.WriteString("(build and flash a removable stick directly). See the design spec for the field\n")
	b.WriteString("checklist (Secure Boot off, Ethernet recommended for FCOS, x86_64 only).\n")
	return b.String()
}
