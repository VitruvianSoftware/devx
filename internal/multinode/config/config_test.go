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
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	content := `
cluster:
  name: test-cluster
  k3sVersion: "v1.31.2+k3s1"
  kubeconfig: "~/.kube/test.yaml"
  docker:
    enabled: true
nodes:
  - host: host-1
    role: server
    pool: cp-1
    vm:
      cpus: 2
      memory: 4GiB
      disk: 30GiB
  - host: host-2
    role: server
    pool: cp-2
    vm:
      cpus: 2
      memory: 4GiB
      disk: 30GiB
  - host: host-3
    role: server
    pool: cp-3
    vm:
      cpus: 2
      memory: 4GiB
      disk: 30GiB
  - host: host-4
    role: agent
    pool: worker-1
    vm:
      cpus: 4
      memory: 8GiB
      disk: 50GiB
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Cluster.Name != "test-cluster" {
		t.Errorf("expected cluster name 'test-cluster', got %q", cfg.Cluster.Name)
	}
	if !cfg.Cluster.Docker.Enabled {
		t.Error("expected cluster.docker.enabled to be true")
	}
	if len(cfg.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(cfg.Nodes))
	}
}

func TestLoad_MetalLBL2Hostnames(t *testing.T) {
	content := `
cluster:
  name: l2-test
  metallb:
    enabled: true
    ipRange: 10.44.86.210-10.44.86.215
    l2Hostnames:
      - fedora
      - nuc9
nodes:
  - host: host-1
    role: server
    pool: cp-1
    vm:
      cpus: 1
      memory: 1GiB
      disk: 10GiB
`
	path := writeTemp(t, content)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := cfg.Cluster.MetalLB.L2Hostnames
	if len(got) != 2 || got[0] != "fedora" || got[1] != "nuc9" {
		t.Errorf("expected l2Hostnames [fedora nuc9], got %v", got)
	}
}

// LBIPRange / LBL2Hostnames must prefer the backend-neutral loadBalancer.* fields
// and fall back to the legacy metallb.* fields — so existing configs keep working
// and a Cilium-LB-IPAM cluster can drop the metallb block entirely while still
// getting its VIPs advertised over tailscale.
func TestLBResolvers_PreferLoadBalancerFallBackToMetalLB(t *testing.T) {
	// Neutral fields set → used; legacy metallb fields ignored.
	neutral := ClusterConfig{
		LoadBalancer: LoadBalancerConfig{IPRange: "10.0.0.1-10.0.0.9", L2Hostnames: []string{"a"}},
		MetalLB:      MetalLBConfig{IPRange: "10.9.9.1-10.9.9.9", L2Hostnames: []string{"z"}},
	}
	if got := neutral.LBIPRange(); got != "10.0.0.1-10.0.0.9" {
		t.Errorf("LBIPRange neutral = %q, want loadBalancer.ipRange", got)
	}
	if got := neutral.LBL2Hostnames(); len(got) != 1 || got[0] != "a" {
		t.Errorf("LBL2Hostnames neutral = %v, want [a]", got)
	}

	// Only legacy metallb fields set → fall back to them.
	legacy := ClusterConfig{MetalLB: MetalLBConfig{IPRange: "10.9.9.1-10.9.9.9", L2Hostnames: []string{"z"}}}
	if got := legacy.LBIPRange(); got != "10.9.9.1-10.9.9.9" {
		t.Errorf("LBIPRange fallback = %q, want metallb.ipRange", got)
	}
	if got := legacy.LBL2Hostnames(); len(got) != 1 || got[0] != "z" {
		t.Errorf("LBL2Hostnames fallback = %v, want [z]", got)
	}

	// Nothing set → empty / nil.
	if got := (ClusterConfig{}).LBIPRange(); got != "" {
		t.Errorf("LBIPRange empty = %q, want empty", got)
	}
	if got := (ClusterConfig{}).LBL2Hostnames(); len(got) != 0 {
		t.Errorf("LBL2Hostnames empty = %v, want none", got)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_MalformedYAML(t *testing.T) {
	path := writeTemp(t, "{{invalid yaml}}")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestLoad_EnvVar(t *testing.T) {
	content := `
cluster:
  name: env-test
nodes:
  - host: host-1
    role: server
    pool: cp-1
    vm:
      cpus: 1
      memory: 1GiB
      disk: 10GiB
`
	path := writeTemp(t, content)
	t.Setenv("CLUSTER_CONFIG", path)

	cfg, err := Load("cluster.yaml") // default path triggers env var check
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Cluster.Name != "env-test" {
		t.Errorf("expected 'env-test', got %q", cfg.Cluster.Name)
	}
}

func TestServerNodes(t *testing.T) {
	cfg := &Config{
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
			{Host: "b", Role: "agent", Pool: "p2", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
			{Host: "c", Role: "server", Pool: "p3", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}

	servers := cfg.ServerNodes()
	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}
}

func TestAgentNodes(t *testing.T) {
	cfg := &Config{
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
			{Host: "b", Role: "agent", Pool: "p2", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
			{Host: "c", Role: "agent", Pool: "p3", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}

	agents := cfg.AgentNodes()
	if len(agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(agents))
	}
}

func TestInitNode(t *testing.T) {
	cfg := &Config{
		Nodes: []NodeConfig{
			{Host: "first-server", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
			{Host: "second-server", Role: "server", Pool: "p2", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}

	init := cfg.InitNode()
	if init.Host != "first-server" {
		t.Errorf("expected init node 'first-server', got %q", init.Host)
	}
}

func TestGetVMName_Default(t *testing.T) {
	n := NodeConfig{Host: "test"}
	if n.GetVMName() != "k8s-node" {
		t.Errorf("expected default VM name 'k8s-node', got %q", n.GetVMName())
	}
}

func TestGetVMName_Custom(t *testing.T) {
	n := NodeConfig{Host: "test", VMName: "custom-vm"}
	if n.GetVMName() != "custom-vm" {
		t.Errorf("expected 'custom-vm', got %q", n.GetVMName())
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
