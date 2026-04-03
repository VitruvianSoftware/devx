package cmd

import (
	"strings"
	"testing"
)

func TestGenerateMermaid_BasicTopology(t *testing.T) {
	cfg := DevxConfig{
		Name: "demo-app",
		Databases: []DevxConfigDatabase{
			{Engine: "postgres", Port: 5432},
			{Engine: "redis"},
		},
		Services: []DevxConfigService{
			{
				Name:    "api",
				Runtime: "host",
				Port:    8080,
				DependsOn: []DevxConfigDependsOn{
					{Name: "postgres", Condition: "service_healthy"},
					{Name: "redis", Condition: "service_healthy"},
				},
			},
			{
				Name:    "web",
				Runtime: "host",
				Port:    3000,
				DependsOn: []DevxConfigDependsOn{
					{Name: "api", Condition: "service_healthy"},
				},
			},
		},
		Tunnels: []DevxConfigTunnel{
			{Name: "api", Port: 8080},
			{Name: "web", Port: 3000},
		},
	}

	diagram := GenerateMermaid(cfg)

	// Check top-level structure
	if !strings.HasPrefix(diagram, "flowchart TD") {
		t.Error("expected diagram to start with 'flowchart TD'")
	}

	// Check database nodes
	if !strings.Contains(diagram, `postgres[("postgres:5432")]`) {
		t.Errorf("expected postgres database node, got:\n%s", diagram)
	}
	if !strings.Contains(diagram, `redis[("redis")]`) {
		t.Errorf("expected redis database node, got:\n%s", diagram)
	}

	// Check service nodes
	if !strings.Contains(diagram, `api("api:8080")`) {
		t.Errorf("expected api service node, got:\n%s", diagram)
	}
	if !strings.Contains(diagram, `web("web:3000")`) {
		t.Errorf("expected web service node, got:\n%s", diagram)
	}

	// Check dependency edges
	if !strings.Contains(diagram, `postgres -->|"service_healthy"| api`) {
		t.Errorf("expected postgres->api edge, got:\n%s", diagram)
	}
	if !strings.Contains(diagram, `redis -->|"service_healthy"| api`) {
		t.Errorf("expected redis->api edge, got:\n%s", diagram)
	}
	if !strings.Contains(diagram, `api -->|"service_healthy"| web`) {
		t.Errorf("expected api->web edge, got:\n%s", diagram)
	}

	// Check tunnel nodes
	if !strings.Contains(diagram, `tunnel_api{{"🌐 api:8080"}}`) {
		t.Errorf("expected tunnel node for api, got:\n%s", diagram)
	}

	// Check tunnel-to-service edges
	if !strings.Contains(diagram, `api -.->|"expose"| tunnel_api`) {
		t.Errorf("expected expose edge from api to tunnel, got:\n%s", diagram)
	}
}

func TestGenerateMermaid_EmptyConfig(t *testing.T) {
	cfg := DevxConfig{Name: "empty"}
	diagram := GenerateMermaid(cfg)

	if !strings.HasPrefix(diagram, "flowchart TD") {
		t.Error("expected diagram header even for empty config")
	}
	// Should have class definitions but no nodes
	if !strings.Contains(diagram, "classDef db") {
		t.Error("expected class definitions")
	}
}

func TestGenerateMermaid_ServiceWithContainerRuntime(t *testing.T) {
	cfg := DevxConfig{
		Services: []DevxConfigService{
			{Name: "worker", Runtime: "container", Port: 9090},
		},
	}

	diagram := GenerateMermaid(cfg)

	if !strings.Contains(diagram, `worker("worker:9090 [container]")`) {
		t.Errorf("expected container runtime label, got:\n%s", diagram)
	}
}

func TestSanitizeID(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"my-service", "my_service"},
		{"my.service", "my_service"},
		{"simple", "simple"},
		{"a/b", "a_b"},
	}

	for _, tc := range cases {
		got := sanitizeID(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeID(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
