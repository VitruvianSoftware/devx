package bridge

import (
	"testing"
)

func TestNewTunnel(t *testing.T) {
	cfg := TunnelConfig{
		Kubeconfig:  "/tmp/kubeconfig",
		Context:     "test-ctx",
		Namespace:   "staging",
		AgentPod:    "devx-bridge-agent-abc123-xxxx",
		ControlPort: AgentControlPort,
		LocalPort:   8080,
	}

	tunnel := NewTunnel(cfg)

	if tunnel == nil {
		t.Fatal("NewTunnel should not return nil")
	}
	if tunnel.cfg.AgentPod != "devx-bridge-agent-abc123-xxxx" {
		t.Error("tunnel config should preserve agent pod name")
	}
	if tunnel.cfg.ControlPort != 4200 {
		t.Error("tunnel config should preserve control port")
	}
	if tunnel.cfg.LocalPort != 8080 {
		t.Error("tunnel config should preserve local port")
	}
}

func TestTunnel_HealthyBeforeStart(t *testing.T) {
	tunnel := NewTunnel(TunnelConfig{})

	if tunnel.Healthy() {
		t.Error("tunnel should not be healthy before Start()")
	}
}

func TestFindFreePort(t *testing.T) {
	port, err := findFreePort()
	if err != nil {
		t.Fatalf("findFreePort should not error: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("findFreePort returned invalid port: %d", port)
	}

	// Ensure two calls return different ports
	port2, err := findFreePort()
	if err != nil {
		t.Fatalf("second findFreePort should not error: %v", err)
	}
	// Note: race condition possible but extremely unlikely in tests
	if port == port2 {
		t.Logf("warning: two findFreePort calls returned same port %d (rare but possible)", port)
	}
}
