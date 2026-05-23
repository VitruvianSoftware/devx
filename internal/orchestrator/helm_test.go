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
	"path/filepath"
	"testing"
)

func TestHelmArgs(t *testing.T) {
	if got := helmArgs("/kc", "ctx", "ns", "uninstall", "rel"); !equalArgs(got,
		[]string{"--kubeconfig", "/kc", "--kube-context", "ctx", "-n", "ns", "uninstall", "rel"}) {
		t.Errorf("helmArgs(with context) = %v", got)
	}
	// --kube-context omitted when empty; -n omitted when empty.
	if got := helmArgs("/kc", "", "", "version"); !equalArgs(got,
		[]string{"--kubeconfig", "/kc", "version"}) {
		t.Errorf("helmArgs(no context/ns) = %v", got)
	}
}

func TestHelmUpgradeArgs(t *testing.T) {
	got := helmUpgradeArgs("/kc", "ctx", "ns", "myrel", "./chart", []string{"a.yaml", "b.yaml"})
	want := []string{
		"--kubeconfig", "/kc", "--kube-context", "ctx", "-n", "ns",
		"upgrade", "--install", "myrel", "./chart", "-f", "a.yaml", "-f", "b.yaml",
	}
	if !equalArgs(got, want) {
		t.Errorf("helmUpgradeArgs() = %v, want %v", got, want)
	}
}

func TestResolveValuesPaths(t *testing.T) {
	got := resolveValuesPaths("/base", []string{"v.yaml", "/abs/v2.yaml"})
	want := []string{filepath.FromSlash("/base/v.yaml"), filepath.FromSlash("/abs/v2.yaml")}
	if !equalArgs(got, want) {
		t.Errorf("resolveValuesPaths() = %v, want %v", got, want)
	}
	if resolveValuesPaths("/base", nil) != nil {
		t.Error("resolveValuesPaths(_, nil) should be nil")
	}
}
