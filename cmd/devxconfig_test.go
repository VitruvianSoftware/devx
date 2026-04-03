package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveConfig_NoIncludes verifies that a plain devx.yaml (no include block)
// works identically to before Idea 44.
func TestResolveConfig_NoIncludes(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: test-proj
services:
  - name: api
    command: ["go", "run", "./cmd/api"]
    port: 8080
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(yamlPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "test-proj" {
		t.Errorf("expected name 'test-proj', got %q", cfg.Name)
	}
	if len(cfg.Services) != 1 || cfg.Services[0].Name != "api" {
		t.Errorf("expected service 'api', got %+v", cfg.Services)
	}
}

// TestResolveConfig_BasicInclude verifies that an include directive correctly
// merges services from a sibling devx.yaml and sets Dir on each included service.
func TestResolveConfig_BasicInclude(t *testing.T) {
	root := t.TempDir()

	// Create the included project
	siblingDir := filepath.Join(root, "payments-api")
	if err := os.MkdirAll(siblingDir, 0755); err != nil {
		t.Fatal(err)
	}
	siblingYAML := filepath.Join(siblingDir, "devx.yaml")
	if err := os.WriteFile(siblingYAML, []byte(`
name: payments-api
services:
  - name: payments-api
    command: ["go", "run", "./cmd/payments"]
    port: 9090
databases:
  - engine: postgres
    port: 5432
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create the parent (orchestrator) project
	parentYAML := filepath.Join(root, "devx.yaml")
	if err := os.WriteFile(parentYAML, []byte(`
name: my-platform
include:
  - path: ./payments-api/devx.yaml
services:
  - name: web-frontend
    command: ["npm", "run", "dev"]
    port: 3000
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(parentYAML, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 services total
	if len(cfg.Services) != 2 {
		t.Errorf("expected 2 services, got %d: %+v", len(cfg.Services), cfg.Services)
	}

	// Included service should have Dir set to the sibling's directory
	var found bool
	for _, svc := range cfg.Services {
		if svc.Name == "payments-api" {
			found = true
			if svc.Dir != siblingDir {
				t.Errorf("payments-api: expected Dir=%q, got %q", siblingDir, svc.Dir)
			}
		}
	}
	if !found {
		t.Error("payments-api service not found in merged config")
	}

	// Included database should also be present with Dir set
	if len(cfg.Databases) != 1 || cfg.Databases[0].Engine != "postgres" {
		t.Errorf("expected postgres database, got %+v", cfg.Databases)
	}
	if cfg.Databases[0].Dir != siblingDir {
		t.Errorf("postgres db: expected Dir=%q, got %q", siblingDir, cfg.Databases[0].Dir)
	}

	// Parent-scope service should have empty Dir
	for _, svc := range cfg.Services {
		if svc.Name == "web-frontend" && svc.Dir != "" {
			t.Errorf("web-frontend: expected empty Dir, got %q", svc.Dir)
		}
	}
}

// TestResolveConfig_ServiceNameCollision verifies that duplicate service names
// trigger a fail-fast error with a helpful message.
func TestResolveConfig_ServiceNameCollision(t *testing.T) {
	root := t.TempDir()

	siblingDir := filepath.Join(root, "svc-b")
	if err := os.MkdirAll(siblingDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siblingDir, "devx.yaml"), []byte(`
services:
  - name: api
    command: ["python", "app.py"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	parentYAML := filepath.Join(root, "devx.yaml")
	if err := os.WriteFile(parentYAML, []byte(`
name: conflicting-platform
include:
  - path: ./svc-b/devx.yaml
services:
  - name: api
    command: ["go", "run", "./cmd/api"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfig(parentYAML, "")
	if err == nil {
		t.Fatal("expected error for duplicate service name, got nil")
	}
	if !contains(err.Error(), "conflict: service \"api\"") {
		t.Errorf("expected conflict error message, got: %v", err)
	}
}

// TestResolveConfig_MissingIncludePath verifies a clear error for missing files.
func TestResolveConfig_MissingIncludePath(t *testing.T) {
	dir := t.TempDir()
	parentYAML := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(parentYAML, []byte(`
name: test
include:
  - path: ./does-not-exist/devx.yaml
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfig(parentYAML, "")
	if err == nil {
		t.Fatal("expected error for missing include path, got nil")
	}
	if !contains(err.Error(), "no such file") {
		t.Errorf("expected 'no such file' in error, got: %v", err)
	}
}

// TestResolveConfig_CircularInclude verifies that circular includes are caught
// by the depth limiter.
func TestResolveConfig_CircularInclude(t *testing.T) {
	root := t.TempDir()

	// a.yaml includes b.yaml, b.yaml includes a.yaml
	aPath := filepath.Join(root, "a.yaml")
	bPath := filepath.Join(root, "b.yaml")

	if err := os.WriteFile(aPath, []byte(`
name: project-a
include:
  - path: ./b.yaml
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, []byte(`
name: project-b
include:
  - path: ./a.yaml
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Should either be deduplicated silently or hit depth limit — must not infinite-loop
	_, err := resolveConfig(aPath, "")
	// Either it succeeds (deduplication) or fails (depth limit).
	// Either way it must not panic or loop forever.
	_ = err
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
