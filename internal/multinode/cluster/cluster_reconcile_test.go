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

package cluster

import (
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/lima"
)

func TestNodePackagesIncludesSocat(t *testing.T) {
	// socat is the reason reconcile exists: kubectl port-forward and devx
	// bridge need it on Docker-runtime k3s nodes. Guard against it being
	// dropped from the baseline.
	if !strings.Contains(lima.NodePackages, "socat") {
		t.Errorf("lima.NodePackages = %q, must include socat for port-forward/bridge", lima.NodePackages)
	}
}

func TestCiliumExtraRoutes(t *testing.T) {
	// Cilium native routing over tailscale: every node advertises its pod CIDR,
	// and the stable L2 nodes ALSO advertise the LoadBalancer /24 so the LB VIPs are
	// tailnet-reachable. Legacy metallb.* fields are honoured via the LB resolvers.
	cfg := &config.Config{}
	cfg.Cluster.MetalLB.IPRange = "10.44.86.210-10.44.86.215"
	cfg.Cluster.MetalLB.L2Hostnames = []string{"fedora", "nuc9"}

	// A stable node gets the LB range as an extra route.
	got := ciliumExtraRoutes("fedora", cfg)
	if len(got) != 1 || got[0] != "10.44.86.0/24" {
		t.Errorf("ciliumExtraRoutes(fedora) = %v, want [10.44.86.0/24]", got)
	}

	// A non-L2 node advertises only its pod CIDR (no extra routes).
	if got := ciliumExtraRoutes("james-mbp", cfg); len(got) != 0 {
		t.Errorf("ciliumExtraRoutes(james-mbp) = %v, want none", got)
	}

	// No LB range => no LB route even for an L2 node.
	cfg2 := &config.Config{}
	cfg2.Cluster.MetalLB.L2Hostnames = []string{"fedora"}
	if got := ciliumExtraRoutes("fedora", cfg2); len(got) != 0 {
		t.Errorf("ciliumExtraRoutes with no ipRange = %v, want none", got)
	}

	// Backend-neutral config (Cilium LB-IPAM, no metallb block): the LB range +
	// L2 nodes come from cluster.loadBalancer, and the route still advertises.
	cfg3 := &config.Config{}
	cfg3.Cluster.LoadBalancer.IPRange = "10.44.86.210-10.44.86.215"
	cfg3.Cluster.LoadBalancer.L2Hostnames = []string{"fedora", "nuc9i5"}
	if got := ciliumExtraRoutes("nuc9i5", cfg3); len(got) != 1 || got[0] != "10.44.86.0/24" {
		t.Errorf("ciliumExtraRoutes(nuc9i5, loadBalancer.*) = %v, want [10.44.86.0/24]", got)
	}
	if got := ciliumExtraRoutes("james-mbp", cfg3); len(got) != 0 {
		t.Errorf("ciliumExtraRoutes(james-mbp, loadBalancer.*) = %v, want none", got)
	}
}

func TestNodeChanges(t *testing.T) {
	vm := config.VMConfig{CPUs: 2, Memory: "4GiB", Disk: "30GiB"}
	matchingSpec := lima.Spec{CPUs: 2, Memory: "4GiB", Disk: "30GiB", Mounts: []config.MountConfig{{Location: "~", Writable: true}}}
	desiredHome := lima.ResolveMounts(nil) // [{~ writable}]

	cases := []struct {
		name              string
		spec              lima.Spec
		vm                config.VMConfig
		desired           []config.MountConfig
		wantHW, wantMount bool
	}{
		{"all equal -> no change", matchingSpec, vm, desiredHome, false, false},
		{"cpu differs", lima.Spec{CPUs: 4, Memory: "4GiB", Disk: "30GiB", Mounts: desiredHome}, vm, desiredHome, true, false},
		{"memory differs", lima.Spec{CPUs: 2, Memory: "8GiB", Disk: "30GiB", Mounts: desiredHome}, vm, desiredHome, true, false},
		{"disk differs", lima.Spec{CPUs: 2, Memory: "4GiB", Disk: "50GiB", Mounts: desiredHome}, vm, desiredHome, true, false},
		// The reported bug: an existing VM with NO mounts vs the default home mount.
		{"no mounts vs default home", lima.Spec{CPUs: 2, Memory: "4GiB", Disk: "30GiB", Mounts: nil}, vm, desiredHome, false, true},
		{"both hw and mounts differ", lima.Spec{CPUs: 1, Memory: "2GiB", Disk: "10GiB", Mounts: nil}, vm, desiredHome, true, true},
	}
	for _, c := range cases {
		hw, mounts := nodeChanges(c.spec, c.vm, c.desired)
		if hw != c.wantHW || mounts != c.wantMount {
			t.Errorf("%s: nodeChanges = (hw=%v, mounts=%v), want (hw=%v, mounts=%v)", c.name, hw, mounts, c.wantHW, c.wantMount)
		}
	}
}
