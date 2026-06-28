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

package provider

import (
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

// #369: specFor is the single adapter point between config.* and the providers.
// It must resolve node identity + the cluster-level fields the providers use, so
// provider methods never reach into config.NodeConfig / config.Config directly.
func TestSpecFor_MapsNodeAndClusterFields(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cluster.Tailscale.Enabled = true
	cfg.Cluster.Tailscale.AuthKey = "tskey-xyz"
	node := config.NodeConfig{Host: "fedora", Role: "server", Pool: "linux", Kind: "native", NodeIP: "100.0.0.1"}

	s := specFor(node, cfg)

	if s.Host != "fedora" || s.Role != "server" || s.Pool != "linux" || s.Kind != "native" || s.NodeIP != "100.0.0.1" {
		t.Errorf("node identity not mapped: %+v", s)
	}
	if !s.Tailscale.Enabled || s.Tailscale.AuthKey != "tskey-xyz" {
		t.Errorf("cluster Tailscale not mapped: %+v", s.Tailscale)
	}
	if s.VMName == "" {
		t.Error("VMName should be resolved via GetVMName (defaults to k8s-node)")
	}
}
