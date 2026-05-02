package config

import "testing"

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 2, Memory: "4GiB", Disk: "30GiB"}},
			{Host: "b", Role: "server", Pool: "p2", VM: VMConfig{CPUs: 2, Memory: "4GiB", Disk: "30GiB"}},
			{Host: "c", Role: "server", Pool: "p3", VM: VMConfig{CPUs: 2, Memory: "4GiB", Disk: "30GiB"}},
			{Host: "d", Role: "agent", Pool: "p4", VM: VMConfig{CPUs: 4, Memory: "8GiB", Disk: "50GiB"}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingClusterName(t *testing.T) {
	cfg := &Config{
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing cluster name")
	}
}

func TestValidate_NoNodes(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for no nodes")
	}
}

func TestValidate_NoServerNodes(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "agent", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for no server nodes")
	}
}

func TestValidate_EvenServerCount(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
			{Host: "b", Role: "server", Pool: "p2", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for even server count")
	}
}

func TestValidate_SingleServer(t *testing.T) {
	// A single server is valid (no HA but still works).
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("single server should be valid, got: %v", err)
	}
}

func TestValidate_InvalidRole(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "master", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid role")
	}
}

func TestValidate_DuplicateHosts(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "same-host", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
			{Host: "same-host", Role: "agent", Pool: "p2", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for duplicate hosts")
	}
}

func TestValidate_DuplicatePools(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "same-pool", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
			{Host: "b", Role: "agent", Pool: "same-pool", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for duplicate pools")
	}
}

func TestValidate_MissingHost(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing host")
	}
}

func TestValidate_MissingPool(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", VM: VMConfig{CPUs: 1, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing pool")
	}
}

func TestValidate_MissingVMMemory(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 1, Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing VM memory")
	}
}

func TestValidate_ZeroCPUs(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "test"},
		Nodes: []NodeConfig{
			{Host: "a", Role: "server", Pool: "p1", VM: VMConfig{CPUs: 0, Memory: "1GiB", Disk: "10GiB"}},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for zero CPUs")
	}
}
