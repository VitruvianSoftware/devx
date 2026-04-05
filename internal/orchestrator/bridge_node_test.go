package orchestrator

import (
	"context"
	"strings"
	"testing"
)

func TestStartBridgeNode_MissingConfig(t *testing.T) {
	node := &Node{
		Name:    "test-bridge",
		Runtime: RuntimeBridge,
	}

	err := startBridgeNode(context.Background(), node)
	if err == nil {
		t.Fatal("expected error when BridgeConfig is nil, got nil")
	}
	if !strings.Contains(err.Error(), "has no BridgeConfig") {
		t.Errorf("expected missing config error, got %v", err)
	}
}

func TestBridgeServicesJSON(t *testing.T) {
	dag := NewDAG()
	_ = dag.AddNode(&Node{
		Name:    "bridge-api",
		Runtime: RuntimeBridge,
		BridgeConfig: &BridgeNodeConfig{
			Mode:          BridgeModeConnect,
			TargetService: "api-svc",
			Namespace:     "staging",
			RemotePort:    8080,
			LocalPort:     8080,
		},
	})
	_ = dag.AddNode(&Node{
		Name:    "host-api",
		Runtime: RuntimeHost,
	}) // should be ignored

	data, err := dag.BridgeServicesJSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, `"name": "bridge-api"`) {
		t.Errorf("expected bridge-api in JSON, got %s", out)
	}
	if !strings.Contains(out, `"target_service": "api-svc"`) {
		t.Errorf("expected api-svc in JSON, got %s", out)
	}
	if strings.Contains(out, "host-api") {
		t.Errorf("expected host-api to be omitted from JSON, got %s", out)
	}
}

func TestBridgeCleanupNil(t *testing.T) {
	// Should not panic
	var s *BridgeNodeState
	s.Cleanup()
}
