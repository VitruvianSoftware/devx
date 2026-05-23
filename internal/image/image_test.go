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

import "testing"

func TestSpecDefaults(t *testing.T) {
	var empty Spec
	if got := empty.TagOrDefault(); got != "dev" {
		t.Errorf("empty TagOrDefault() = %q, want dev", got)
	}
	if got := empty.ContextOrDefault(); got != "." {
		t.Errorf("empty ContextOrDefault() = %q, want .", got)
	}
	if got := empty.DockerfileOrDefault(); got != "Dockerfile" {
		t.Errorf("empty DockerfileOrDefault() = %q, want Dockerfile", got)
	}

	set := Spec{Tag: "v1", Context: "./svc", Dockerfile: "Dockerfile.dev"}
	if got := set.TagOrDefault(); got != "v1" {
		t.Errorf("set TagOrDefault() = %q, want v1", got)
	}
	if got := set.ContextOrDefault(); got != "./svc" {
		t.Errorf("set ContextOrDefault() = %q, want ./svc", got)
	}
	if got := set.DockerfileOrDefault(); got != "Dockerfile.dev" {
		t.Errorf("set DockerfileOrDefault() = %q, want Dockerfile.dev", got)
	}
}

func TestLocalRegistryRefs(t *testing.T) {
	r := &LocalRegistry{PushAddr: "100.98.214.120:30500", RefPrefix: "localhost:30500"}
	if got := r.Ref("brms-offer", "dev"); got != "localhost:30500/brms-offer:dev" {
		t.Errorf("Ref() = %q, want localhost:30500/brms-offer:dev", got)
	}
	if got := r.PushRef("brms-offer", "dev"); got != "100.98.214.120:30500/brms-offer:dev" {
		t.Errorf("PushRef() = %q, want 100.98.214.120:30500/brms-offer:dev", got)
	}
}

func TestKubectlArgs(t *testing.T) {
	got := kubectlArgs("/tmp/kc", "ctx", "get", "nodes")
	want := []string{"--kubeconfig", "/tmp/kc", "--context", "ctx", "get", "nodes"}
	if len(got) != len(want) {
		t.Fatalf("kubectlArgs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kubectlArgs() = %v, want %v", got, want)
		}
	}
	// context omitted when empty
	if g := kubectlArgs("/tmp/kc", "", "x"); len(g) != 3 || g[2] != "x" {
		t.Errorf("kubectlArgs(no ctx) = %v, want [--kubeconfig /tmp/kc x]", g)
	}
}
