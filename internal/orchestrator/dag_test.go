package orchestrator

import (
	"testing"
)

func TestTopologicalSort_Linear(t *testing.T) {
	dag := NewDAG()

	dag.AddNode(&Node{Name: "postgres", Type: NodeDatabase})
	dag.AddNode(&Node{Name: "api", Type: NodeService, DependsOn: []string{"postgres"}})
	dag.AddNode(&Node{Name: "web", Type: NodeService, DependsOn: []string{"api"}})

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

	dag.AddNode(&Node{Name: "postgres", Type: NodeDatabase})
	dag.AddNode(&Node{Name: "redis", Type: NodeDatabase})
	dag.AddNode(&Node{Name: "api", Type: NodeService, DependsOn: []string{"postgres", "redis"}})

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

	dag.AddNode(&Node{Name: "a", Type: NodeService, DependsOn: []string{"b"}})
	dag.AddNode(&Node{Name: "b", Type: NodeService, DependsOn: []string{"a"}})

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

	dag.AddNode(&Node{Name: "api", Type: NodeService, DependsOn: []string{"nonexistent"}})

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

	dag.AddNode(&Node{Name: "a", Type: NodeService})
	dag.AddNode(&Node{Name: "b", Type: NodeService})
	dag.AddNode(&Node{Name: "c", Type: NodeService})

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
	dag.AddNode(&Node{Name: "db", Type: NodeDatabase})
	dag.AddNode(&Node{Name: "api", Type: NodeService, DependsOn: []string{"db"}})
	dag.AddNode(&Node{Name: "worker", Type: NodeService, DependsOn: []string{"db"}})
	dag.AddNode(&Node{Name: "gateway", Type: NodeService, DependsOn: []string{"api", "worker"}})

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
