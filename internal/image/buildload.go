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

package image

import (
	"context"
	"fmt"

	"github.com/VitruvianSoftware/devx/internal/provider"
)

// BuildAndLoad ensures the in-cluster registry exists, then builds and pushes each
// spec's image into it, so that runtime: kubernetes manifests can reference
// localhost:<NodePort>/<name>:<tag>. providerName selects the container backend to
// build with (e.g. "lima", "podman"; "" or "auto" auto-detects) — pass devx's
// resolved provider so multi-backend hosts don't fail to auto-detect. baseDir is
// the service's working directory (build contexts resolve against it);
// kubeconfig/kctx target the cluster (kctx may be empty for the current context).
// It is a no-op when specs is empty.
func BuildAndLoad(ctx context.Context, providerName, baseDir, kubeconfig, kctx string, specs []Spec) error {
	if len(specs) == 0 {
		return nil
	}
	for _, s := range specs {
		if s.Name == "" {
			return fmt.Errorf("kubernetes.images: every image entry needs a name")
		}
	}

	prov, err := provider.GetProvider(providerName)
	if err != nil {
		return fmt.Errorf("resolving a container runtime to build images: %w", err)
	}
	rt := prov.Runtime

	reg, err := EnsureRegistry(ctx, kubeconfig, kctx)
	if err != nil {
		return err
	}
	fmt.Printf("  🗄️  in-cluster registry ready (push %s · ref %s)\n", reg.PushAddr, reg.RefPrefix)

	for _, s := range specs {
		tag := s.TagOrDefault()
		pushRef := reg.PushRef(s.Name, tag)
		fmt.Printf("  🔨 building %s (%s)...\n", s.Name, rt.Name())
		if err := Build(ctx, rt, baseDir, s, pushRef); err != nil {
			return err
		}
		if err := Push(ctx, rt, pushRef); err != nil {
			return err
		}
		fmt.Printf("  📦 loaded %s — reference it in manifests as %s\n", s.Name, reg.Ref(s.Name, tag))
	}
	return nil
}
