package bridge

import (
	"strings"
	"testing"
)

func TestGenerateAgentManifest_ContainsNamedPorts(t *testing.T) {
	cfg := AgentConfig{
		Namespace:     "staging",
		TargetService: "payments-api",
		ServicePorts: []ServicePortSpec{
			{Name: "http-api", Port: 8080, TargetPort: "http-api", Protocol: "TCP"},
			{Name: "metrics", Port: 9090, TargetPort: "9090", Protocol: "TCP"},
		},
		OriginalSelector: map[string]string{"app": "payments"},
		AgentImage:        AgentImageDefault,
		SessionID:         "abc12345",
		Deadline:          14400,
	}

	manifest := generateAgentManifest(cfg)

	// Verify named ports are in the manifest
	if !strings.Contains(manifest, "name: http-api") {
		t.Error("manifest should contain named port 'http-api'")
	}
	if !strings.Contains(manifest, "name: metrics") {
		t.Error("manifest should contain named port 'metrics'")
	}
	if !strings.Contains(manifest, "containerPort: 8080") {
		t.Error("manifest should contain containerPort 8080")
	}
	if !strings.Contains(manifest, "containerPort: 9090") {
		t.Error("manifest should contain containerPort 9090")
	}
}

func TestGenerateAgentManifest_ContainsControlAndHealthPorts(t *testing.T) {
	cfg := AgentConfig{
		Namespace:     "default",
		TargetService: "echo",
		ServicePorts: []ServicePortSpec{
			{Port: 5678, Protocol: "TCP"},
		},
		OriginalSelector: map[string]string{"app": "echo"},
		AgentImage:        AgentImageDefault,
		SessionID:         "test1234",
		Deadline:          14400,
	}

	manifest := generateAgentManifest(cfg)

	if !strings.Contains(manifest, "containerPort: 4200") {
		t.Error("manifest should contain control port 4200")
	}
	if !strings.Contains(manifest, "containerPort: 4201") {
		t.Error("manifest should contain health port 4201")
	}
}

func TestGenerateAgentManifest_ContainsServiceAccount(t *testing.T) {
	cfg := AgentConfig{
		Namespace:     "default",
		TargetService: "echo",
		ServicePorts: []ServicePortSpec{
			{Port: 5678, Protocol: "TCP"},
		},
		OriginalSelector: map[string]string{"app": "echo"},
		AgentImage:        AgentImageDefault,
		SessionID:         "sa-test",
		Deadline:          14400,
	}

	manifest := generateAgentManifest(cfg)

	if !strings.Contains(manifest, "kind: ServiceAccount") {
		t.Error("manifest should contain ServiceAccount")
	}
	if !strings.Contains(manifest, "kind: Role") {
		t.Error("manifest should contain Role")
	}
	if !strings.Contains(manifest, "kind: RoleBinding") {
		t.Error("manifest should contain RoleBinding")
	}
	if !strings.Contains(manifest, "serviceAccountName: devx-bridge-agent-sa-test") {
		t.Error("manifest should reference the correct ServiceAccount")
	}
}

func TestGenerateAgentManifest_ContainsSelfHealingEnvVars(t *testing.T) {
	cfg := AgentConfig{
		Namespace:        "staging",
		TargetService:    "payments",
		ServicePorts:     []ServicePortSpec{{Port: 8080, Protocol: "TCP"}},
		OriginalSelector: map[string]string{"app": "payments", "version": "v2"},
		AgentImage:       AgentImageDefault,
		SessionID:        "heal-test",
		Deadline:         14400,
	}

	manifest := generateAgentManifest(cfg)

	if !strings.Contains(manifest, "DEVX_ORIGINAL_SELECTOR") {
		t.Error("manifest should contain DEVX_ORIGINAL_SELECTOR env var")
	}
	if !strings.Contains(manifest, "DEVX_TARGET_SERVICE") {
		t.Error("manifest should contain DEVX_TARGET_SERVICE env var")
	}
	if !strings.Contains(manifest, "DEVX_TARGET_NAMESPACE") {
		t.Error("manifest should contain DEVX_TARGET_NAMESPACE env var")
	}
	if !strings.Contains(manifest, `"app":"payments"`) {
		t.Error("manifest should contain the original selector JSON")
	}
}

func TestGenerateAgentManifest_RBACScoped(t *testing.T) {
	cfg := AgentConfig{
		Namespace:        "staging",
		TargetService:    "payments-api",
		ServicePorts:     []ServicePortSpec{{Port: 8080, Protocol: "TCP"}},
		OriginalSelector: map[string]string{"app": "payments"},
		AgentImage:       AgentImageDefault,
		SessionID:        "rbac-test",
		Deadline:         14400,
	}

	manifest := generateAgentManifest(cfg)

	// Verify the Role is scoped to the specific target service
	if !strings.Contains(manifest, `resourceNames: ["payments-api"]`) {
		t.Error("Role should be scoped to the target service via resourceNames")
	}
}

func TestGenerateAgentManifest_PortArgs(t *testing.T) {
	cfg := AgentConfig{
		Namespace:     "default",
		TargetService: "multi-port-svc",
		ServicePorts: []ServicePortSpec{
			{Name: "http", Port: 8080, Protocol: "TCP"},
			{Name: "grpc", Port: 9090, Protocol: "TCP"},
		},
		OriginalSelector: map[string]string{"app": "test"},
		AgentImage:        AgentImageDefault,
		SessionID:         "ports-test",
		Deadline:          14400,
	}

	manifest := generateAgentManifest(cfg)

	if !strings.Contains(manifest, "--ports=8080:http,9090:grpc") {
		t.Errorf("manifest should contain correct port args, got:\n%s", manifest)
	}
}

func TestAgentDefaultConfig(t *testing.T) {
	if AgentControlPort != 4200 {
		t.Errorf("expected control port 4200, got %d", AgentControlPort)
	}
	if AgentHealthPort != 4201 {
		t.Errorf("expected health port 4201, got %d", AgentHealthPort)
	}
	if AgentDefaultDeadline != 14400 {
		t.Errorf("expected default deadline 14400, got %d", AgentDefaultDeadline)
	}
}
