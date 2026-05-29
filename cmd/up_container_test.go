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

package cmd

import (
	"testing"

	"github.com/VitruvianSoftware/devx/internal/image"
)

func TestToContainerNodeConfig_Image(t *testing.T) {
	svc := DevxConfigService{
		Name:      "api",
		Runtime:   "container",
		Container: &DevxConfigServiceContainer{Image: "myorg/api:dev", Args: []string{"--cap-add=NET_ADMIN"}},
	}
	cfg := toContainerNodeConfig(svc, "lima")
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Image != "myorg/api:dev" {
		t.Errorf("image = %q", cfg.Image)
	}
	if cfg.Build != nil {
		t.Errorf("expected nil build, got %+v", cfg.Build)
	}
	if cfg.ProviderName != "lima" {
		t.Errorf("provider = %q", cfg.ProviderName)
	}
	if len(cfg.Args) != 1 || cfg.Args[0] != "--cap-add=NET_ADMIN" {
		t.Errorf("args = %v", cfg.Args)
	}
}

func TestToContainerNodeConfig_Build(t *testing.T) {
	svc := DevxConfigService{
		Name:    "web",
		Runtime: "container",
		Container: &DevxConfigServiceContainer{
			Build: &DevxConfigContainerBuild{Context: "./web", Dockerfile: "Dockerfile.dev", Tag: "local", Platforms: []string{"linux/arm64"}},
		},
	}
	cfg := toContainerNodeConfig(svc, "lima")
	want := &image.Spec{Name: "web", Context: "./web", Dockerfile: "Dockerfile.dev", Tag: "local", Platforms: []string{"linux/arm64"}}
	if cfg.Build == nil {
		t.Fatal("expected build spec, got nil")
	}
	if cfg.Build.Name != want.Name || cfg.Build.Context != want.Context ||
		cfg.Build.Dockerfile != want.Dockerfile || cfg.Build.Tag != want.Tag {
		t.Errorf("build spec = %+v, want %+v", cfg.Build, want)
	}
}

func TestToContainerNodeConfig_NilWhenNoBlock(t *testing.T) {
	svc := DevxConfigService{Name: "api", Runtime: "container"}
	if cfg := toContainerNodeConfig(svc, "lima"); cfg != nil {
		t.Errorf("expected nil when container block absent, got %+v", cfg)
	}
}
