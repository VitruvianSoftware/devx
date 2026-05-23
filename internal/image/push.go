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

	"github.com/VitruvianSoftware/devx/internal/provider"
)

// pushArgs assembles the `<runtime> push` args for ref, adding the runtime's
// insecure-registry flag so pushes to the in-cluster HTTP registry succeed.
// docker has no per-push insecure flag (it relies on daemon `insecure-registries`),
// so none is added for it — a docker-runtime host needs that configured separately.
func pushArgs(runtime, ref string) []string {
	args := []string{"push"}
	switch runtime {
	case "podman":
		args = append(args, "--tls-verify=false")
	case "nerdctl":
		args = append(args, "--insecure-registry")
	}
	return append(args, ref)
}

// Push pushes ref to the registry over an insecure connection, streaming output.
func Push(ctx context.Context, rt provider.ContainerRuntime, ref string) error {
	cmd := rt.CommandContext(ctx, pushArgs(rt.Name(), ref)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pushing image %s with %s: %w", ref, rt.Name(), err)
	}
	return nil
}
