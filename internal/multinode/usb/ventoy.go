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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirSink is an ArtifactSink that writes artifacts as files under Root.
type DirSink struct {
	Root string
}

// Add implements ArtifactSink. Paths use forward slashes and are written
// relative to Root; parent directories are created as needed.
func (d DirSink) Add(a Artifact) error {
	if strings.Contains(a.Path, "..") {
		return fmt.Errorf("artifact path %q must not contain parent-directory references", a.Path)
	}
	mode := a.Mode
	if mode == 0 {
		mode = 0o644
	}
	full := filepath.Join(d.Root, filepath.FromSlash(a.Path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("creating dir for %s: %w", a.Path, err)
	}
	if err := os.WriteFile(full, a.Contents, mode); err != nil {
		return fmt.Errorf("writing %s: %w", a.Path, err)
	}
	return nil
}

// ventoyControl is one control directive in ventoy.json.
type ventoyControl map[string]string

// ventoyAlias maps an image file to a custom menu title.
type ventoyAlias struct {
	Image string `json:"image"`
	Alias string `json:"alias"`
}

// ventoyAutoInstall maps an image to one or more autoinstall/preseed templates.
// With more than one template Ventoy prompts the operator to choose at boot —
// this is how the ephemeral vs install entries surface for a single ISO.
type ventoyAutoInstall struct {
	Image    string   `json:"image"`
	Template []string `json:"template"`
}

// ventoyConfig is the subset of ventoy.json devx generates.
type ventoyConfig struct {
	Control     []ventoyControl     `json:"control"`
	MenuAlias   []ventoyAlias       `json:"menu_alias"`
	AutoInstall []ventoyAutoInstall `json:"auto_install,omitempty"`
}

// VentoyJSON renders a deterministic ventoy.json from the boot entries. It
// aliases each base image and, for cloud-init/autoinstall entries, registers
// their templates so Ventoy offers the ephemeral/install choice at boot.
//
// Note: ventoy.json is a field-tunable starting point. FCOS Ignition is
// delivered by embedding it into the live ISO (coreos-installer iso ignition
// embed) at stick-prep time, so it needs no Ventoy plugin entry.
func VentoyJSON(entries []BootEntry) ([]byte, error) {
	// Group templates by base image (sorted, deduped) for determinism.
	aliasByImage := map[string]string{}
	templatesByImage := map[string][]string{}

	for _, e := range entries {
		if e.BaseImage == "" {
			continue
		}
		img := "/" + e.BaseImage
		// Alias is the renderer family, without the per-mode suffix.
		aliasByImage[img] = "devx node — " + rendererFamily(e.Renderer)
		if e.Injection.Kind == InjectCloudInit && e.Injection.PayloadPath != "" {
			tpl := "/" + e.Injection.PayloadPath
			templatesByImage[img] = appendUnique(templatesByImage[img], tpl)
		}
	}

	cfg := ventoyConfig{
		Control: []ventoyControl{
			{"VTOY_MENU_TIMEOUT": "10"},
			{"VTOY_DEFAULT_MENU_MODE": "0"},
		},
	}

	images := sortedKeys(aliasByImage)
	for _, img := range images {
		cfg.MenuAlias = append(cfg.MenuAlias, ventoyAlias{Image: img, Alias: aliasByImage[img]})
	}
	for _, img := range sortedKeys(templatesByImage) {
		tpls := templatesByImage[img]
		sort.Strings(tpls)
		cfg.AutoInstall = append(cfg.AutoInstall, ventoyAutoInstall{Image: img, Template: tpls})
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// rendererFamily maps a renderer name to a human label for menu aliases.
func rendererFamily(name string) string {
	switch name {
	case "fcos":
		return "Fedora CoreOS"
	case "ubuntu":
		return "Ubuntu"
	case "baked":
		return "prebaked image"
	default:
		return name
	}
}

func appendUnique(s []string, v string) []string {
	for _, e := range s {
		if e == v {
			return s
		}
	}
	return append(s, v)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
