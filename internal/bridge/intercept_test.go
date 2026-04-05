package bridge

import (
	"testing"
)

func TestValidateInterceptable_RejectsExternalName(t *testing.T) {
	info := &ServiceInfo{
		Name:      "my-service",
		Namespace: "default",
		Type:      "ExternalName",
		Selector:  map[string]string{"app": "test"},
		Ports: []ServicePortSpec{
			{Port: 80, Protocol: "TCP"},
		},
	}

	err := ValidateInterceptable(info)
	if err == nil {
		t.Fatal("expected error for ExternalName service, got nil")
	}
	if err.Error() == "" || !contains(err.Error(), "ExternalName") {
		t.Errorf("error should mention ExternalName, got: %s", err.Error())
	}
}

func TestValidateInterceptable_RejectsEmptySelector(t *testing.T) {
	info := &ServiceInfo{
		Name:      "headless-svc",
		Namespace: "default",
		Type:      "ClusterIP",
		Selector:  map[string]string{},
		Ports: []ServicePortSpec{
			{Port: 80, Protocol: "TCP"},
		},
	}

	err := ValidateInterceptable(info)
	if err == nil {
		t.Fatal("expected error for empty selector, got nil")
	}
	if !contains(err.Error(), "no selector") {
		t.Errorf("error should mention 'no selector', got: %s", err.Error())
	}
}

func TestValidateInterceptable_RejectsNilSelector(t *testing.T) {
	info := &ServiceInfo{
		Name:      "no-selector-svc",
		Namespace: "default",
		Type:      "ClusterIP",
		Selector:  nil,
		Ports: []ServicePortSpec{
			{Port: 80, Protocol: "TCP"},
		},
	}

	err := ValidateInterceptable(info)
	if err == nil {
		t.Fatal("expected error for nil selector, got nil")
	}
}

func TestValidateInterceptable_RejectsUDP(t *testing.T) {
	info := &ServiceInfo{
		Name:      "dns-service",
		Namespace: "kube-system",
		Type:      "ClusterIP",
		Selector:  map[string]string{"app": "dns"},
		Ports: []ServicePortSpec{
			{Name: "dns", Port: 53, Protocol: "UDP"},
		},
	}

	err := ValidateInterceptable(info)
	if err == nil {
		t.Fatal("expected error for UDP port, got nil")
	}
	if !contains(err.Error(), "UDP") {
		t.Errorf("error should mention UDP, got: %s", err.Error())
	}
}

func TestValidateInterceptable_AcceptsTCP(t *testing.T) {
	info := &ServiceInfo{
		Name:      "web-service",
		Namespace: "default",
		Type:      "ClusterIP",
		Selector:  map[string]string{"app": "web"},
		Ports: []ServicePortSpec{
			{Name: "http-api", Port: 8080, TargetPort: "http-api", Protocol: "TCP"},
			{Name: "metrics", Port: 9090, TargetPort: "9090", Protocol: "TCP"},
		},
	}

	err := ValidateInterceptable(info)
	if err != nil {
		t.Fatalf("expected no error for valid TCP service, got: %v", err)
	}
}

func TestValidateInterceptable_AcceptsMixedCaseProtocol(t *testing.T) {
	info := &ServiceInfo{
		Name:      "mixed-case",
		Namespace: "default",
		Type:      "ClusterIP",
		Selector:  map[string]string{"app": "test"},
		Ports: []ServicePortSpec{
			{Port: 80, Protocol: "tcp"},
		},
	}

	err := ValidateInterceptable(info)
	if err != nil {
		t.Fatalf("expected no error for lowercase 'tcp' protocol, got: %v", err)
	}
}

func TestValidateInterceptable_RejectsUDPAmongTCPPorts(t *testing.T) {
	info := &ServiceInfo{
		Name:      "mixed-protocol",
		Namespace: "default",
		Type:      "ClusterIP",
		Selector:  map[string]string{"app": "test"},
		Ports: []ServicePortSpec{
			{Name: "http", Port: 80, Protocol: "TCP"},
			{Name: "dns", Port: 53, Protocol: "UDP"},
		},
	}

	err := ValidateInterceptable(info)
	if err == nil {
		t.Fatal("expected error for service with UDP port, got nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
