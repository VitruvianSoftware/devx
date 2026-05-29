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

package lima

import (
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

func TestResolveMounts(t *testing.T) {
	t.Run("nil -> default home writable", func(t *testing.T) {
		got := ResolveMounts(nil)
		want := []config.MountConfig{{Location: "~", Writable: true}}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
	t.Run("empty -> none", func(t *testing.T) {
		got := ResolveMounts([]config.MountConfig{})
		if len(got) != 0 {
			t.Fatalf("got %#v, want empty", got)
		}
	})
	t.Run("explicit -> as-is", func(t *testing.T) {
		in := []config.MountConfig{{Location: "/work", Writable: true}}
		got := ResolveMounts(in)
		if len(got) != 1 || got[0] != in[0] {
			t.Fatalf("got %#v, want %#v", got, in)
		}
	})
}

func TestRenderMounts(t *testing.T) {
	t.Run("default home", func(t *testing.T) {
		out := renderMounts(ResolveMounts(nil))
		for _, want := range []string{`mountType: "virtiofs"`, "mounts:", `- location: "~"`, "writable: true"} {
			if !strings.Contains(out, want) {
				t.Errorf("renderMounts() = %q, missing %q", out, want)
			}
		}
	})
	t.Run("no mounts renders nothing", func(t *testing.T) {
		if out := renderMounts(nil); out != "" {
			t.Errorf("renderMounts(nil) = %q, want empty", out)
		}
		if out := renderMounts([]config.MountConfig{}); out != "" {
			t.Errorf("renderMounts([]) = %q, want empty", out)
		}
	})
	t.Run("mountPoint only emitted when different", func(t *testing.T) {
		same := renderMounts([]config.MountConfig{{Location: "/x", MountPoint: "/x"}})
		if strings.Contains(same, "mountPoint") {
			t.Errorf("equal mountPoint should be omitted, got %q", same)
		}
		diff := renderMounts([]config.MountConfig{{Location: "/x", MountPoint: "/y"}})
		if !strings.Contains(diff, `mountPoint: "/y"`) {
			t.Errorf("differing mountPoint should be emitted, got %q", diff)
		}
	})
}

func TestParseSpec(t *testing.T) {
	y := `vmType: "vz"
cpus: 2
memory: "4GiB"
disk: "30GiB"
mountType: "virtiofs"
mounts:
  - location: "~"
    writable: true
networks:
  - socket: "/x"
`
	spec, err := ParseSpec(y)
	if err != nil {
		t.Fatal(err)
	}
	if spec.CPUs != 2 || spec.Memory != "4GiB" || spec.Disk != "30GiB" {
		t.Fatalf("hw mismatch: %#v", spec)
	}
	if len(spec.Mounts) != 1 || spec.Mounts[0].Location != "~" || !spec.Mounts[0].Writable {
		t.Fatalf("mounts mismatch: %#v", spec.Mounts)
	}
}

func TestParseSpec_NoMounts(t *testing.T) {
	spec, err := ParseSpec("cpus: 1\nmemory: \"2GiB\"\ndisk: \"10GiB\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Mounts) != 0 {
		t.Fatalf("want no mounts, got %#v", spec.Mounts)
	}
}

func TestMountsEqual(t *testing.T) {
	home := []config.MountConfig{{Location: "~", Writable: true}}
	cases := []struct {
		name string
		a, b []config.MountConfig
		want bool
	}{
		{"both empty", nil, []config.MountConfig{}, true},
		{"equal home", home, []config.MountConfig{{Location: "~", Writable: true}}, true},
		{"empty mountPoint normalizes", []config.MountConfig{{Location: "/x"}}, []config.MountConfig{{Location: "/x", MountPoint: "/x"}}, true},
		{"missing on live side", []config.MountConfig{}, home, false},
		{"writable differs", []config.MountConfig{{Location: "~"}}, home, false},
		{"length differs", home, append(home, config.MountConfig{Location: "/y"}), false},
	}
	for _, c := range cases {
		if got := MountsEqual(c.a, c.b); got != c.want {
			t.Errorf("%s: MountsEqual = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMountsRewriteScript(t *testing.T) {
	s := MountsRewriteScript("k8s-node", ResolveMounts(nil))
	for _, want := range []string{
		"cd ~/.lima/k8s-node",
		"/^mounts:/ { inblock=1; next }",
		"/^mountType:/ { next }",
		"lima.yaml.devx.tmp",
		`mountType: "virtiofs"`,
		`- location: "~"`,
		"mv lima.yaml.devx.tmp lima.yaml",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("MountsRewriteScript missing %q in:\n%s", want, s)
		}
	}
	empty := MountsRewriteScript("k8s-node", []config.MountConfig{})
	if strings.Contains(empty, "location:") {
		t.Errorf("opt-out script should append no mounts, got:\n%s", empty)
	}
}

func TestGenerateConfig_Mounts(t *testing.T) {
	m := &Manager{node: config.NodeConfig{VM: config.VMConfig{CPUs: 2, Memory: "4GiB", Disk: "30GiB"}}, vmName: "k8s-node"}

	t.Run("default home mount when nil", func(t *testing.T) {
		out := m.GenerateConfig("/sock", true, nil)
		for _, want := range []string{`mountType: "virtiofs"`, "mounts:", `- location: "~"`, "writable: true"} {
			if !strings.Contains(out, want) {
				t.Errorf("GenerateConfig missing %q in:\n%s", want, out)
			}
		}
	})

	t.Run("opt-out renders no mounts block", func(t *testing.T) {
		out := m.GenerateConfig("/sock", true, []config.MountConfig{})
		if strings.Contains(out, "mounts:") || strings.Contains(out, "mountType:") {
			t.Errorf("opt-out should render no mounts/mountType, got:\n%s", out)
		}
	})

	t.Run("docker socket still rendered alongside mounts", func(t *testing.T) {
		out := m.GenerateConfig("/sock", true, nil)
		if !strings.Contains(out, "/var/run/docker.sock") {
			t.Errorf("expected docker portForward, got:\n%s", out)
		}
	})
}
