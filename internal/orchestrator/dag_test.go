package orchestrator

import (
	"testing"
)

func TestTopologicalSort_Linear(t *testing.T) {
	dag := NewDAG()

	_ = dag.AddNode(&Node{Name: "postgres", Type: NodeDatabase})
	_ = dag.AddNode(&Node{Name: "api", Type: NodeService, DependsOn: []string{"postgres"}})
	_ = dag.AddNode(&Node{Name: "web", Type: NodeService, DependsOn: []string{"api"}})

	tiers, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() failed: %v", err)
	}

	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %d: %v", len(tiers), tiers)
	}

	if tiers[0][0] != "postgres" {
		t.Errorf("tier 0: expected [postgres], got %v", tiers[0])
	}
	if tiers[1][0] != "api" {
		t.Errorf("tier 1: expected [api], got %v", tiers[1])
	}
	if tiers[2][0] != "web" {
		t.Errorf("tier 2: expected [web], got %v", tiers[2])
	}
}

func TestTopologicalSort_Parallel(t *testing.T) {
	dag := NewDAG()

	_ = dag.AddNode(&Node{Name: "postgres", Type: NodeDatabase})
	_ = dag.AddNode(&Node{Name: "redis", Type: NodeDatabase})
	_ = dag.AddNode(&Node{Name: "api", Type: NodeService, DependsOn: []string{"postgres", "redis"}})

	tiers, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() failed: %v", err)
	}

	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d: %v", len(tiers), tiers)
	}

	// Tier 0 should have both postgres and redis (parallel)
	if len(tiers[0]) != 2 {
		t.Errorf("tier 0: expected 2 nodes, got %d: %v", len(tiers[0]), tiers[0])
	}
}

func TestTopologicalSort_CycleDetection(t *testing.T) {
	dag := NewDAG()

	_ = dag.AddNode(&Node{Name: "a", Type: NodeService, DependsOn: []string{"b"}})
	_ = dag.AddNode(&Node{Name: "b", Type: NodeService, DependsOn: []string{"a"}})

	_, err := dag.TopologicalSort()
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}

	if !contains(err.Error(), "cycle") {
		t.Errorf("expected error to mention 'cycle', got: %v", err)
	}
}

func TestValidate_MissingDependency(t *testing.T) {
	dag := NewDAG()

	_ = dag.AddNode(&Node{Name: "api", Type: NodeService, DependsOn: []string{"nonexistent"}})

	err := dag.Validate()
	if err == nil {
		t.Fatal("expected validation error for missing dependency, got nil")
	}

	if !contains(err.Error(), "unknown node") {
		t.Errorf("expected error to mention 'unknown node', got: %v", err)
	}
}

func TestTopologicalSort_NoDependencies(t *testing.T) {
	dag := NewDAG()

	_ = dag.AddNode(&Node{Name: "a", Type: NodeService})
	_ = dag.AddNode(&Node{Name: "b", Type: NodeService})
	_ = dag.AddNode(&Node{Name: "c", Type: NodeService})

	tiers, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() failed: %v", err)
	}

	// All independent nodes should be in a single tier
	if len(tiers) != 1 {
		t.Fatalf("expected 1 tier, got %d: %v", len(tiers), tiers)
	}
	if len(tiers[0]) != 3 {
		t.Errorf("expected 3 nodes in tier 0, got %d", len(tiers[0]))
	}
}

func TestTopologicalSort_DiamondDependency(t *testing.T) {
	dag := NewDAG()

	// Diamond: db -> api, db -> worker, api+worker -> gateway
	_ = dag.AddNode(&Node{Name: "db", Type: NodeDatabase})
	_ = dag.AddNode(&Node{Name: "api", Type: NodeService, DependsOn: []string{"db"}})
	_ = dag.AddNode(&Node{Name: "worker", Type: NodeService, DependsOn: []string{"db"}})
	_ = dag.AddNode(&Node{Name: "gateway", Type: NodeService, DependsOn: []string{"api", "worker"}})

	tiers, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() failed: %v", err)
	}

	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %d: %v", len(tiers), tiers)
	}

	if tiers[0][0] != "db" {
		t.Errorf("tier 0: expected [db], got %v", tiers[0])
	}
	if len(tiers[1]) != 2 {
		t.Errorf("tier 1: expected 2 parallel nodes, got %d: %v", len(tiers[1]), tiers[1])
	}
	if tiers[2][0] != "gateway" {
		t.Errorf("tier 2: expected [gateway], got %v", tiers[2])
	}
}

func TestAddNode_Duplicate(t *testing.T) {
	dag := NewDAG()

	if err := dag.AddNode(&Node{Name: "api", Type: NodeService}); err != nil {
		t.Fatalf("first AddNode failed: %v", err)
	}

	err := dag.AddNode(&Node{Name: "api", Type: NodeService})
	if err == nil {
		t.Fatal("expected duplicate error, got nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ─── Idea 46.3: Bridge DAG Tests ─────────────────────────────────────────────

func TestTopologicalSort_BridgeBeforeService(t *testing.T) {
	dag := NewDAG()

	// Bridge node should be sorted before the service that depends on it
	_ = dag.AddNode(&Node{
		Name:       "remote-payments",
		Type:       NodeService,
		Runtime:    RuntimeBridge,
		BridgeMode: BridgeModeConnect,
		BridgeConfig: &BridgeNodeConfig{
			TargetService: "payments-api",
			Namespace:     "staging",
			RemotePort:    8080,
			LocalPort:     8080,
			Mode:          BridgeModeConnect,
		},
	})
	_ = dag.AddNode(&Node{
		Name:      "local-api",
		Type:      NodeService,
		Runtime:   RuntimeHost,
		DependsOn: []string{"remote-payments"},
		Command:   []string{"go", "run", "./cmd/api"},
	})

	tiers, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() failed: %v", err)
	}

	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d: %v", len(tiers), tiers)
	}
	if tiers[0][0] != "remote-payments" {
		t.Errorf("tier 0: expected [remote-payments], got %v", tiers[0])
	}
	if tiers[1][0] != "local-api" {
		t.Errorf("tier 1: expected [local-api], got %v", tiers[1])
	}
}

func TestTopologicalSort_InterceptDependsOnLocalService(t *testing.T) {
	dag := NewDAG()

	// Intercept depends on local service (must start local first, then steal traffic)
	_ = dag.AddNode(&Node{
		Name:    "local-user-svc",
		Type:    NodeService,
		Runtime: RuntimeHost,
		Command: []string{"go", "run", "./cmd/user"},
		Port:    3000,
	})
	_ = dag.AddNode(&Node{
		Name:       "user-intercept",
		Type:       NodeService,
		Runtime:    RuntimeBridge,
		BridgeMode: BridgeModeIntercept,
		DependsOn:  []string{"local-user-svc"},
		BridgeConfig: &BridgeNodeConfig{
			TargetService: "user-service",
			Namespace:     "staging",
			RemotePort:    3000,
			LocalPort:     3000,
			Mode:          BridgeModeIntercept,
			InterceptMode: "steal",
		},
	})

	tiers, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() failed: %v", err)
	}

	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d: %v", len(tiers), tiers)
	}
	if tiers[0][0] != "local-user-svc" {
		t.Errorf("tier 0: expected [local-user-svc], got %v", tiers[0])
	}
	if tiers[1][0] != "user-intercept" {
		t.Errorf("tier 1: expected [user-intercept], got %v", tiers[1])
	}
}

func TestTopologicalSort_MultipleBridgesParallel(t *testing.T) {
	dag := NewDAG()

	// Two independent bridge nodes should be in the same tier
	_ = dag.AddNode(&Node{
		Name: "bridge-a", Type: NodeService, Runtime: RuntimeBridge, BridgeMode: BridgeModeConnect,
		BridgeConfig: &BridgeNodeConfig{TargetService: "svc-a", RemotePort: 8080, Mode: BridgeModeConnect},
	})
	_ = dag.AddNode(&Node{
		Name: "bridge-b", Type: NodeService, Runtime: RuntimeBridge, BridgeMode: BridgeModeConnect,
		BridgeConfig: &BridgeNodeConfig{TargetService: "svc-b", RemotePort: 9090, Mode: BridgeModeConnect},
	})

	tiers, err := dag.TopologicalSort()
	if err != nil {
		t.Fatalf("TopologicalSort() failed: %v", err)
	}

	if len(tiers) != 1 {
		t.Fatalf("expected 1 tier (parallel), got %d: %v", len(tiers), tiers)
	}
	if len(tiers[0]) != 2 {
		t.Errorf("expected 2 parallel bridge nodes, got %d", len(tiers[0]))
	}
}

func TestBridgeEnvVars(t *testing.T) {
	dag := NewDAG()

	node := &Node{
		Name:       "remote-payments",
		Type:       NodeService,
		Runtime:    RuntimeBridge,
		BridgeMode: BridgeModeConnect,
		Port:       8080,
		BridgeConfig: &BridgeNodeConfig{
			TargetService: "payments-api",
			Namespace:     "staging",
			RemotePort:    8080,
			LocalPort:     8080,
			Mode:          BridgeModeConnect,
		},
	}
	// Simulate post-execution state
	node.bridgeState = &BridgeNodeState{}
	_ = dag.AddNode(node)

	envs := dag.GenerateBridgeEnvVars()
	expected := "http://localhost:8080"
	if envs["BRIDGE_PAYMENTS_API_URL"] != expected {
		t.Errorf("expected BRIDGE_PAYMENTS_API_URL=%q, got %q", expected, envs["BRIDGE_PAYMENTS_API_URL"])
	}
}

func TestToEnvName(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"payments-api", "PAYMENTS_API"},
		{"user.service", "USER_SERVICE"},
		{"redis", "REDIS"},
		{"ALREADY-UPPER", "ALREADY_UPPER"},
	}
	for _, tc := range tests {
		got := toEnvName(tc.input)
		if got != tc.expected {
			t.Errorf("toEnvName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
