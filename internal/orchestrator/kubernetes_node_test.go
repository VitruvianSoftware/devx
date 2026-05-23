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

package orchestrator

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestKubectlArgs(t *testing.T) {
	t.Run("includes context when set", func(t *testing.T) {
		got := kubectlArgs("/tmp/kc", "my-ctx", "apply", "-k", ".")
		want := []string{"--kubeconfig", "/tmp/kc", "--context", "my-ctx", "apply", "-k", "."}
		if !equalArgs(got, want) {
			t.Errorf("kubectlArgs() = %v, want %v", got, want)
		}
	})

	t.Run("omits context when empty", func(t *testing.T) {
		got := kubectlArgs("/tmp/kc", "", "get", "pods")
		want := []string{"--kubeconfig", "/tmp/kc", "get", "pods"}
		if !equalArgs(got, want) {
			t.Errorf("kubectlArgs() = %v, want %v", got, want)
		}
	})

	// Regression: each call must return a fresh slice. A shared backing array
	// would let a second call's append clobber the first call's tail.
	t.Run("returns independent slices per call", func(t *testing.T) {
		a := kubectlArgs("/tmp/kc", "ctx", "apply", "-k", "a")
		b := kubectlArgs("/tmp/kc", "ctx", "delete", "-f", "b")
		if a[len(a)-1] != "a" {
			t.Errorf("first call's tail was mutated by the second: a=%v", a)
		}
		if b[len(b)-1] != "b" {
			t.Errorf("second call's tail wrong: b=%v", b)
		}
	})
}

func TestApplyFlagForRenderer(t *testing.T) {
	tests := []struct {
		renderer string
		want     string
		wantErr  string // substring; "" means no error expected
	}{
		{"kustomize", "-k", ""},
		{"raw", "-f", ""},
		{"helm", "", "not implemented yet"},
		{"", "", "unknown kubernetes.renderer"},
		{"bogus", "", "unknown kubernetes.renderer"},
	}
	for _, tt := range tests {
		t.Run(tt.renderer, func(t *testing.T) {
			got, err := applyFlagForRenderer(tt.renderer)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tt.want {
					t.Errorf("applyFlagForRenderer(%q) = %q, want %q", tt.renderer, got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestResolveManifestsPath(t *testing.T) {
	t.Run("absolute path returned unchanged", func(t *testing.T) {
		n := &Node{Dir: "/some/where"}
		if got := resolveManifestsPath(n, "/abs/manifests"); got != "/abs/manifests" {
			t.Errorf("got %q, want /abs/manifests", got)
		}
	})

	t.Run("relative path joined against node Dir", func(t *testing.T) {
		n := &Node{Dir: "/repo/root"}
		want := filepath.Join("/repo/root", "deploy/k8s")
		if got := resolveManifestsPath(n, "deploy/k8s"); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestStartKubernetesNode_MissingConfig(t *testing.T) {
	t.Run("nil Kube", func(t *testing.T) {
		n := &Node{Name: "svc", Runtime: RuntimeKubernetes}
		err := startKubernetesNode(context.Background(), n)
		if err == nil || !strings.Contains(err.Error(), "no kubernetes.manifests is set") {
			t.Errorf("expected missing-manifests error, got %v", err)
		}
	})

	t.Run("empty manifests", func(t *testing.T) {
		n := &Node{Name: "svc", Runtime: RuntimeKubernetes, Kube: &KubeNodeConfig{}}
		err := startKubernetesNode(context.Background(), n)
		if err == nil || !strings.Contains(err.Error(), "no kubernetes.manifests is set") {
			t.Errorf("expected missing-manifests error, got %v", err)
		}
	})
}

func TestDeleteKubernetesNode_NoApplied(t *testing.T) {
	// kubeApplied is nil (apply never succeeded) → no-op, must not panic or shell out.
	deleteKubernetesNode(&Node{Name: "svc"})
}

func equalArgs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
