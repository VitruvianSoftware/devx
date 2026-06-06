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
	"os"
	"path/filepath"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

func dryRunCfg() *config.Config {
	return &config.Config{
		Cluster: config.ClusterConfig{K3sVersion: "v1.30.4+k3s1"},
	}
}

func TestBuild_DryRunPlan(t *testing.T) {
	res, err := Build(context.Background(), dryRunCfg(), BuildOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	// 3 renderers × 2 modes.
	if len(res.Entries) != 6 {
		t.Errorf("want 6 boot entries, got %d", len(res.Entries))
	}
	wantImages := []string{BakedImage, FCOSImage, UbuntuImage}
	if len(res.RequiredImages) != len(wantImages) {
		t.Fatalf("want %v, got %v", wantImages, res.RequiredImages)
	}
	for i, img := range wantImages {
		if res.RequiredImages[i] != img {
			t.Errorf("required image[%d] = %s, want %s", i, res.RequiredImages[i], img)
		}
	}
}

func TestBuild_WritesArtifacts(t *testing.T) {
	out := t.TempDir()
	cfg := dryRunCfg()
	// Pre-seed coordinates path: dry-run avoids cluster access, but to write
	// files we must go through the non-dry path. Provide a fake by setting an
	// explicit tailnet URL + agent token via config and a single renderer to
	// keep the test hermetic — ResolveCoordinates still needs a server, so use
	// the renderer/build seam directly instead.
	var sink ArtifactSink = DirSink{Root: out}
	spec := agentSpec()
	if _, err := (FCOSRenderer{}).Render(spec, ModeEphemeral, sink); err != nil {
		t.Fatal(err)
	}
	vj, err := VentoyJSON([]BootEntry{{BaseImage: FCOSImage, Renderer: "fcos", Injection: Injection{Kind: InjectIgnition}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Add(Artifact{Path: "ventoy/ventoy.json", Contents: vj}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "fcos", "ephemeral", "devx-join.bu")); err != nil {
		t.Errorf("expected butane artifact on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "ventoy", "ventoy.json")); err != nil {
		t.Errorf("expected ventoy.json on disk: %v", err)
	}
	_ = cfg
}

func TestParseScratch(t *testing.T) {
	cases := []struct {
		in      string
		want    ScratchStrategy
		src     string
		wantErr bool
	}{
		{"", ScratchNone, "", false},
		{"none", ScratchNone, "", false},
		{"free-space", ScratchFreeSpace, "", false},
		{"existing:LABEL=devx", ScratchExisting, "LABEL=devx", false},
		{"existing:", "", "", true},
		{"garbage", "", "", true},
	}
	for _, tc := range cases {
		got, err := parseScratch(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseScratch(%q): want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseScratch(%q): %v", tc.in, err)
			continue
		}
		if got.Strategy != tc.want || got.Source != tc.src {
			t.Errorf("parseScratch(%q) = %+v, want strat=%s src=%s", tc.in, got, tc.want, tc.src)
		}
	}
}

func TestSelectRenderers(t *testing.T) {
	all, err := selectRenderers(nil, nil)
	if err != nil || len(all) != 3 {
		t.Fatalf("default should be all 3 renderers, got %d (%v)", len(all), err)
	}
	one, err := selectRenderers([]string{"fcos"}, nil)
	if err != nil || len(one) != 1 || one[0].Name() != "fcos" {
		t.Fatalf("explicit flag should win: %v %v", one, err)
	}
	fromCfg, err := selectRenderers(nil, []string{"ubuntu", "baked"})
	if err != nil || len(fromCfg) != 2 {
		t.Fatalf("config fallback should yield 2, got %d (%v)", len(fromCfg), err)
	}
	if _, err := selectRenderers([]string{"bogus"}, nil); err == nil {
		t.Error("unknown renderer must error")
	}
}
