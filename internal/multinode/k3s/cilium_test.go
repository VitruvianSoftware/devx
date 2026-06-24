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
	"context"
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
)

// ReadNodePodCIDR reads a node's assigned pod CIDR off the cluster via kubectl
// jsonpath (run on a control-plane node that has kubectl).
func TestReadNodePodCIDR(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("cp-1"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		return "10.42.3.0/24\n", nil
	})
	cidr, err := m.ReadNodePodCIDR(context.Background(), "fedora")
	if err != nil {
		t.Fatalf("ReadNodePodCIDR: %v", err)
	}
	if cidr != "10.42.3.0/24" {
		t.Errorf("podCIDR = %q, want 10.42.3.0/24", cidr)
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		"k3s kubectl get node fedora",
		"jsonpath='{.spec.podCIDR}'",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("ReadNodePodCIDR missing %q in:\n%s", want, joined)
		}
	}
}

// AdvertiseNodeRoutes runs `tailscale set --advertise-routes=... --accept-routes=true`
// on the node. With no extra routes it advertises only the pod CIDR.
func TestAdvertiseNodeRoutes_PodCIDROnly(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("worker-1"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		return "", nil
	})
	if err := m.AdvertiseNodeRoutes(context.Background(), "10.42.5.0/24", nil); err != nil {
		t.Fatalf("AdvertiseNodeRoutes: %v", err)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "tailscale set --advertise-routes=10.42.5.0/24 --accept-routes=true") {
		t.Errorf("AdvertiseNodeRoutes pod-CIDR-only wrong; got:\n%s", joined)
	}
	if strings.Contains(joined, ",") {
		t.Errorf("no extra routes => no comma-list expected; got:\n%s", joined)
	}
}

// With extra routes (the LB range), they are appended to --advertise-routes after
// the pod CIDR as a comma-separated list.
func TestAdvertiseNodeRoutes_PodCIDRPlusLBRange(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("fedora"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		return "", nil
	})
	if err := m.AdvertiseNodeRoutes(context.Background(), "10.42.3.0/24", []string{"10.44.86.0/24"}); err != nil {
		t.Fatalf("AdvertiseNodeRoutes: %v", err)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "tailscale set --advertise-routes=10.42.3.0/24,10.44.86.0/24 --accept-routes=true") {
		t.Errorf("AdvertiseNodeRoutes pod-CIDR+LB-range wrong; got:\n%s", joined)
	}
}

// LBRangeCIDR derives the /24 carrying a MetalLB IP range (whether given as a
// dashed range like 10.44.86.210-10.44.86.215, a bare-suffix dashed range like
// 10.44.86.210-215, or already a CIDR) so the LB IPs are advertised as a tailnet
// subnet route.
func TestLBRangeCIDR(t *testing.T) {
	cases := map[string]string{
		"10.44.86.210-10.44.86.215": "10.44.86.0/24",
		"10.44.86.210-215":          "10.44.86.0/24",
		"10.44.86.0/24":             "10.44.86.0/24",
		"10.44.86.210":              "10.44.86.0/24",
	}
	for in, want := range cases {
		if got := LBRangeCIDR(in); got != want {
			t.Errorf("LBRangeCIDR(%q) = %q, want %q", in, got, want)
		}
	}
	if got := LBRangeCIDR(""); got != "" {
		t.Errorf("LBRangeCIDR(\"\") = %q, want empty", got)
	}
}
