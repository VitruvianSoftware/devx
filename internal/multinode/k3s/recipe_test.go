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

package k3s

import (
	"strings"
	"testing"
)

func mustContainAll(t *testing.T, name, s string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(s, w) {
			t.Errorf("%s missing %q in:\n%s", name, w, s)
		}
	}
}

func mustContainNone(t *testing.T, name, s string, bad ...string) {
	t.Helper()
	for _, b := range bad {
		if strings.Contains(s, b) {
			t.Errorf("%s must NOT contain %q in:\n%s", name, b, s)
		}
	}
}

// #368: install recipes rendered from pure builders, asserted without a host.
func TestBuildServerInitScript(t *testing.T) {
	s := buildServerInitScript("fedora", "10.0.0.1", "linux", "v1.31.2+k3s1",
		[]string{"san1", "san2"}, true, true, false, true)
	mustContainAll(t, "init script", s,
		"https://get.k3s.io", `INSTALL_K3S_VERSION="v1.31.2+k3s1"`, "--cluster-init",
		"--node-name=fedora", "--node-ip=10.0.0.1", "--advertise-address=10.0.0.1",
		"--tls-san=san1", "--tls-san=san2", "--disable servicelb",
		"--flannel-iface=tailscale0", "--flannel-backend=none", "--disable-kube-proxy",
		"--disable metrics-server",
		"--node-label=pool=linux")
	mustContainNone(t, "init script", s, "--docker", "--flannel-iface=lima0")
}

func TestBuildServerJoinScript(t *testing.T) {
	// empty version + docker runtime + lima iface + no servicelb/cilium.
	s := buildServerJoinScript("mbp", "10.0.0.2", "https://cp:6443", "tok", "linux", "",
		[]string{"s1"}, false, false, true, false)
	mustContainAll(t, "join-server script", s,
		"INSTALL_K3S_SKIP_START=true", `K3S_TOKEN="tok"`, "--server=https://cp:6443",
		"--node-name=mbp", "--flannel-iface=lima0", "--docker", "--tls-san=s1")
	mustContainNone(t, "join-server script", s,
		"INSTALL_K3S_VERSION", "--disable servicelb", "--flannel-backend=none", "--cluster-init",
		"--disable metrics-server")
}

func TestBuildAgentJoinScript_NoServerOnlyFlags(t *testing.T) {
	s := buildAgentJoinScript("nuc9", "10.0.0.3", "https://cp:6443", "tok", "linux", "v1.31.2+k3s1", true, false)
	mustContainAll(t, "agent script", s,
		"sh -s - agent", `K3S_URL="https://cp:6443"`, "--node-name=nuc9",
		"--flannel-iface=tailscale0", "--node-label=pool=linux")
	// Agents must NOT carry server-only flags (the agent CLI rejects them).
	mustContainNone(t, "agent script", s,
		"--flannel-backend=none", "--disable-kube-proxy", "--disable-network-policy",
		"--cluster-init", "--disable servicelb", "--docker", "--disable metrics-server")
}
