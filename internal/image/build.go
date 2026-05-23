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
	"os"
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/provider"
)

// buildArgs assembles the `<runtime> build` argument list for spec, tagging the
// result as ref. contextDir and dockerfile must already be resolved to absolute
// paths. Exposed (lowercase, package-internal) for unit testing the arg shape.
func buildArgs(contextDir, dockerfile string, platforms []string, ref string) []string {
	args := []string{"build", "-t", ref, "-f", dockerfile}
	if len(platforms) > 0 {
		args = append(args, "--platform", strings.Join(platforms, ","))
	}
	return append(args, contextDir)
}

// resolvePaths resolves spec's build context and Dockerfile to absolute paths,
// relative to baseDir (the service's working directory) when not already absolute.
func resolvePaths(baseDir string, spec Spec) (contextDir, dockerfile string) {
	contextDir = spec.ContextOrDefault()
	if !filepath.IsAbs(contextDir) {
		contextDir = filepath.Join(baseDir, contextDir)
	}
	dockerfile = spec.DockerfileOrDefault()
	if !filepath.IsAbs(dockerfile) {
		dockerfile = filepath.Join(contextDir, dockerfile)
	}
	return contextDir, dockerfile
}

// Build builds spec's image with the given container runtime and tags it as ref.
// Build context and Dockerfile are resolved relative to baseDir. Build output is
// streamed to the user's terminal.
func Build(ctx context.Context, rt provider.ContainerRuntime, baseDir string, spec Spec, ref string) error {
	contextDir, dockerfile := resolvePaths(baseDir, spec)
	cmd := rt.CommandContext(ctx, buildArgs(contextDir, dockerfile, spec.Platforms, ref)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building image %s with %s: %w", ref, rt.Name(), err)
	}
	return nil
}
