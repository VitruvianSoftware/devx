package config_test

import (
	"testing"

	"github.com/VitruvianSoftware/devx/internal/config"
)

func TestNew_Defaults(t *testing.T) {
	cfg := config.New("james", "", "", "")

	if cfg.DevHostname != "james-dev-machine" {
		t.Errorf("DevHostname = %q, want %q", cfg.DevHostname, "james-dev-machine")
	}
	if cfg.CFDomain != "james.ipv1337.dev" {
		t.Errorf("CFDomain = %q, want %q", cfg.CFDomain, "james.ipv1337.dev")
	}
	if cfg.TunnelName != "dev-tunnel-james" {
		t.Errorf("TunnelName = %q, want %q", cfg.TunnelName, "dev-tunnel-james")
	}
}

func TestNew_ExplicitOverrides(t *testing.T) {
	cfg := config.New("james", "custom-host", "custom-tunnel", "custom.domain.dev")

	if cfg.DevHostname != "custom-host" {
		t.Errorf("DevHostname = %q, want %q", cfg.DevHostname, "custom-host")
	}
	if cfg.TunnelName != "custom-tunnel" {
		t.Errorf("TunnelName = %q, want %q", cfg.TunnelName, "custom-tunnel")
	}
	if cfg.CFDomain != "custom.domain.dev" {
		t.Errorf("CFDomain = %q, want %q", cfg.CFDomain, "custom.domain.dev")
	}
}

func TestValidate_MissingDevName(t *testing.T) {
	cfg := &config.Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty DevName, got nil")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := config.New("james", "", "", "")
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
