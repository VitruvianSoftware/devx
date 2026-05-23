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
	"path/filepath"
	"testing"
)

func TestBuildArgs(t *testing.T) {
	got := buildArgs("/abs/ctx", "/abs/ctx/Dockerfile", nil, "localhost:30500/x:dev")
	want := []string{"build", "-t", "localhost:30500/x:dev", "-f", "/abs/ctx/Dockerfile", "/abs/ctx"}
	assertArgs(t, got, want)

	gotP := buildArgs("/c", "/c/Dockerfile", []string{"linux/amd64", "linux/arm64"}, "ref")
	wantP := []string{"build", "-t", "ref", "-f", "/c/Dockerfile", "--platform", "linux/amd64,linux/arm64", "/c"}
	assertArgs(t, gotP, wantP)
}

func TestPushArgs(t *testing.T) {
	assertArgs(t, pushArgs("podman", "r/x:dev"), []string{"push", "--tls-verify=false", "r/x:dev"})
	assertArgs(t, pushArgs("nerdctl", "r/x:dev"), []string{"push", "--insecure-registry", "r/x:dev"})
	assertArgs(t, pushArgs("docker", "r/x:dev"), []string{"push", "r/x:dev"})
}

func TestResolvePaths(t *testing.T) {
	// Relative context + default Dockerfile → joined under baseDir.
	c, d := resolvePaths("/base", Spec{Context: "svc"})
	if c != filepath.FromSlash("/base/svc") {
		t.Errorf("contextDir = %q, want /base/svc", c)
	}
	if d != filepath.FromSlash("/base/svc/Dockerfile") {
		t.Errorf("dockerfile = %q, want /base/svc/Dockerfile", d)
	}
	// Absolute context preserved; explicit Dockerfile honored.
	c2, d2 := resolvePaths("/base", Spec{Context: "/abs", Dockerfile: "Dockerfile.dev"})
	if c2 != filepath.FromSlash("/abs") {
		t.Errorf("contextDir = %q, want /abs", c2)
	}
	if d2 != filepath.FromSlash("/abs/Dockerfile.dev") {
		t.Errorf("dockerfile = %q, want /abs/Dockerfile.dev", d2)
	}
}

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %v, want %v", got, want)
		}
	}
}
