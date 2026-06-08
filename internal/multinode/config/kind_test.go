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

package config

import "testing"

func TestGetKind_DefaultsToLima(t *testing.T) {
	n := NodeConfig{}
	if n.GetKind() != "lima" {
		t.Errorf("empty kind should default to lima, got %q", n.GetKind())
	}
	n.Kind = "native"
	if n.GetKind() != "native" {
		t.Errorf("explicit kind should be returned, got %q", n.GetKind())
	}
}

func TestValidate_NativeNodeSkipsVM(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "c"},
		Nodes: []NodeConfig{
			{Host: "cp", Role: "server", Pool: "p", VM: VMConfig{CPUs: 2, Memory: "4GiB", Disk: "40GiB"}},
			{Host: "fedora", Kind: "native", Role: "agent", Pool: "linux"}, // no VM fields
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("native node must not require vm fields: %v", err)
	}
}

func TestValidate_BadKind(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "c"},
		Nodes:   []NodeConfig{{Host: "x", Kind: "windows", Role: "agent", Pool: "p"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("invalid kind must be rejected")
	}
}
