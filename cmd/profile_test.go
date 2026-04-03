package cmd

import (
	"testing"
)

func TestMergeProfile_AddNewService(t *testing.T) {
	cfg := DevxConfig{
		Services: []DevxConfigService{
			{Name: "api", Runtime: "host", Port: 8080},
		},
	}

	profile := DevxConfigProfile{
		Services: []DevxConfigService{
			{Name: "worker", Runtime: "host", Port: 9090, Command: []string{"go", "run", "./cmd/worker"}},
		},
	}

	mergeProfile(&cfg, profile)

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}
	if cfg.Services[1].Name != "worker" {
		t.Errorf("expected new service 'worker', got %q", cfg.Services[1].Name)
	}
}

func TestMergeProfile_OverrideExistingService(t *testing.T) {
	cfg := DevxConfig{
		Services: []DevxConfigService{
			{Name: "api", Runtime: "host", Port: 8080, Env: map[string]string{"LOG_LEVEL": "info"}},
		},
	}

	profile := DevxConfigProfile{
		Services: []DevxConfigService{
			{Name: "api", Port: 9090, Env: map[string]string{"LOG_LEVEL": "debug", "NEW_VAR": "yes"}},
		},
	}

	mergeProfile(&cfg, profile)

	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service (merged), got %d", len(cfg.Services))
	}
	if cfg.Services[0].Port != 9090 {
		t.Errorf("expected port 9090 (profile override), got %d", cfg.Services[0].Port)
	}
	if cfg.Services[0].Runtime != "host" {
		t.Errorf("expected runtime 'host' (base preserved), got %q", cfg.Services[0].Runtime)
	}
	if cfg.Services[0].Env["LOG_LEVEL"] != "debug" {
		t.Errorf("expected LOG_LEVEL=debug (profile override), got %q", cfg.Services[0].Env["LOG_LEVEL"])
	}
	if cfg.Services[0].Env["NEW_VAR"] != "yes" {
		t.Errorf("expected NEW_VAR=yes (profile addition), got %q", cfg.Services[0].Env["NEW_VAR"])
	}
}

func TestMergeProfile_AddNewDatabase(t *testing.T) {
	cfg := DevxConfig{
		Databases: []DevxConfigDatabase{
			{Engine: "postgres", Port: 5432},
		},
	}

	profile := DevxConfigProfile{
		Databases: []DevxConfigDatabase{
			{Engine: "mysql", Port: 3306},
		},
	}

	mergeProfile(&cfg, profile)

	if len(cfg.Databases) != 2 {
		t.Fatalf("expected 2 databases, got %d", len(cfg.Databases))
	}
	if cfg.Databases[1].Engine != "mysql" {
		t.Errorf("expected new database 'mysql', got %q", cfg.Databases[1].Engine)
	}
}

func TestMergeProfile_OverrideDatabase(t *testing.T) {
	cfg := DevxConfig{
		Databases: []DevxConfigDatabase{
			{Engine: "postgres", Port: 5432},
		},
	}

	profile := DevxConfigProfile{
		Databases: []DevxConfigDatabase{
			{Engine: "postgres", Port: 5433},
		},
	}

	mergeProfile(&cfg, profile)

	if len(cfg.Databases) != 1 {
		t.Fatalf("expected 1 database (merged), got %d", len(cfg.Databases))
	}
	if cfg.Databases[0].Port != 5433 {
		t.Errorf("expected port 5433 (profile override), got %d", cfg.Databases[0].Port)
	}
}

func TestMergeProfile_OverrideTunnel(t *testing.T) {
	cfg := DevxConfig{
		Tunnels: []DevxConfigTunnel{
			{Name: "api", Port: 8080, Throttle: "3g"},
		},
	}

	profile := DevxConfigProfile{
		Tunnels: []DevxConfigTunnel{
			{Name: "api", Port: 9090},
		},
	}

	mergeProfile(&cfg, profile)

	if len(cfg.Tunnels) != 1 {
		t.Fatalf("expected 1 tunnel (merged), got %d", len(cfg.Tunnels))
	}
	if cfg.Tunnels[0].Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Tunnels[0].Port)
	}
	if cfg.Tunnels[0].Throttle != "3g" {
		t.Errorf("expected throttle '3g' preserved from base, got %q", cfg.Tunnels[0].Throttle)
	}
}

func TestMergeProfile_EmptyProfile(t *testing.T) {
	cfg := DevxConfig{
		Services:  []DevxConfigService{{Name: "api", Port: 8080}},
		Databases: []DevxConfigDatabase{{Engine: "postgres", Port: 5432}},
	}

	profile := DevxConfigProfile{}
	mergeProfile(&cfg, profile)

	if len(cfg.Services) != 1 || len(cfg.Databases) != 1 {
		t.Fatal("empty profile should not modify config")
	}
}
