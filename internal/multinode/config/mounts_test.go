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

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// The default-vs-opt-out behavior hinges on yaml.v3 distinguishing an omitted
// key (nil slice) from an explicitly empty list (non-nil, len 0). Guard it.
func TestClusterMountsUnmarshal_NilVsEmptyVsExplicit(t *testing.T) {
	t.Run("omitted -> nil", func(t *testing.T) {
		var c ClusterConfig
		if err := yaml.Unmarshal([]byte("name: c\n"), &c); err != nil {
			t.Fatal(err)
		}
		if c.Mounts != nil {
			t.Fatalf("omitted mounts: got %#v, want nil", c.Mounts)
		}
	})

	t.Run("empty list -> non-nil len 0", func(t *testing.T) {
		var c ClusterConfig
		if err := yaml.Unmarshal([]byte("name: c\nmounts: []\n"), &c); err != nil {
			t.Fatal(err)
		}
		if c.Mounts == nil {
			t.Fatal("explicit empty mounts: got nil, want non-nil len 0")
		}
		if len(c.Mounts) != 0 {
			t.Fatalf("explicit empty mounts: got len %d, want 0", len(c.Mounts))
		}
	})

	t.Run("explicit entry parsed", func(t *testing.T) {
		var c ClusterConfig
		y := "name: c\nmounts:\n  - location: /work\n    mountPoint: /w\n    writable: true\n"
		if err := yaml.Unmarshal([]byte(y), &c); err != nil {
			t.Fatal(err)
		}
		want := MountConfig{Location: "/work", MountPoint: "/w", Writable: true}
		if len(c.Mounts) != 1 || c.Mounts[0] != want {
			t.Fatalf("got %#v, want [%#v]", c.Mounts, want)
		}
	})
}
