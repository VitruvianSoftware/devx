// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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

// ─── Idea 46.3: Bridge Validation Tests ──────────────────────────────────────

func TestValidateBridgeServices_ValidTarget(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-test
bridge:
  kubeconfig: ~/.kube/config
  context: test-context
  namespace: staging
services:
  - name: remote-payments
    runtime: bridge
    bridge_target:
      service: payments-api
      port: 8080
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(yamlPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	if cfg.Services[0].Runtime != "bridge" {
		t.Errorf("expected runtime 'bridge', got %q", cfg.Services[0].Runtime)
	}
	if cfg.Services[0].BridgeTarget == nil {
		t.Fatal("expected BridgeTarget to be set")
	}
	if cfg.Services[0].BridgeTarget.Service != "payments-api" {
		t.Errorf("expected target service 'payments-api', got %q", cfg.Services[0].BridgeTarget.Service)
	}
}

func TestValidateBridgeServices_ValidIntercept(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-test
bridge:
  kubeconfig: ~/.kube/config
  context: test-context
  namespace: staging
services:
  - name: intercept-svc
    runtime: bridge
    bridge_intercept:
      service: user-service
      port: 3000
      local_port: 3000
      mode: steal
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(yamlPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Services[0].BridgeIntercept == nil {
		t.Fatal("expected BridgeIntercept to be set")
	}
	if cfg.Services[0].BridgeIntercept.Mode != "steal" {
		t.Errorf("expected mode 'steal', got %q", cfg.Services[0].BridgeIntercept.Mode)
	}
}

func TestValidateBridgeServices_NoBridgeSection(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-test
services:
  - name: remote-svc
    runtime: bridge
    bridge_target:
      service: payments-api
      port: 8080
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfig(yamlPath, "")
	if err == nil {
		t.Fatal("expected error when bridge section is missing, got nil")
	}
	if !contains(err.Error(), "no top-level 'bridge:' section") {
		t.Errorf("expected bridge section error, got: %v", err)
	}
}

func TestValidateBridgeServices_NeitherTargetNorIntercept(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-test
bridge:
  context: test
services:
  - name: broken
    runtime: bridge
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfig(yamlPath, "")
	if err == nil {
		t.Fatal("expected error for bridge with no target/intercept, got nil")
	}
	if !contains(err.Error(), "neither bridge_target nor bridge_intercept") {
		t.Errorf("expected neither error, got: %v", err)
	}
}

func TestValidateBridgeServices_BothTargetAndIntercept(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-test
bridge:
  context: test
services:
  - name: both
    runtime: bridge
    bridge_target:
      service: svc-a
      port: 8080
    bridge_intercept:
      service: svc-b
      port: 3000
      mode: steal
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfig(yamlPath, "")
	if err == nil {
		t.Fatal("expected error for both target and intercept, got nil")
	}
	if !contains(err.Error(), "cannot have both") {
		t.Errorf("expected 'cannot have both' error, got: %v", err)
	}
}

func TestValidateBridgeServices_MissingTargetService(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-test
bridge:
  context: test
services:
  - name: missing-svc
    runtime: bridge
    bridge_target:
      port: 8080
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfig(yamlPath, "")
	if err == nil {
		t.Fatal("expected error for missing target service name, got nil")
	}
	if !contains(err.Error(), "bridge_target.service is required") {
		t.Errorf("expected service required error, got: %v", err)
	}
}

func TestValidateBridgeServices_InvalidTargetPort(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-test
bridge:
  context: test
services:
  - name: bad-port
    runtime: bridge
    bridge_target:
      service: payments-api
      port: 0
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfig(yamlPath, "")
	if err == nil {
		t.Fatal("expected error for port 0, got nil")
	}
	if !contains(err.Error(), "bridge_target.port must be > 0") {
		t.Errorf("expected port error, got: %v", err)
	}
}

func TestValidateBridgeServices_InvalidInterceptMode(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-test
bridge:
  context: test
services:
  - name: bad-mode
    runtime: bridge
    bridge_intercept:
      service: user-svc
      port: 3000
      mode: mirror
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := resolveConfig(yamlPath, "")
	if err == nil {
		t.Fatal("expected error for mirror mode, got nil")
	}
	if !contains(err.Error(), "mode must be 'steal'") {
		t.Errorf("expected mode error, got: %v", err)
	}
}

func TestValidateBridgeServices_ProfileMerge(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: bridge-profile-test
bridge:
  context: test
  namespace: staging
services:
  - name: api
    runtime: host
    command: ["go", "run", "./cmd/api"]
profiles:
  hybrid:
    services:
      - name: remote-payments
        runtime: bridge
        bridge_target:
          service: payments-api
          port: 8080
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(yamlPath, "hybrid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Profile should add the bridge service
	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services with hybrid profile, got %d: %+v", len(cfg.Services), cfg.Services)
	}

	var found bool
	for _, svc := range cfg.Services {
		if svc.Name == "remote-payments" {
			found = true
			if svc.Runtime != "bridge" {
				t.Errorf("expected runtime 'bridge', got %q", svc.Runtime)
			}
			if svc.BridgeTarget == nil {
				t.Error("expected BridgeTarget to be set after profile merge")
			}
		}
	}
	if !found {
		t.Error("remote-payments service not found after profile merge")
	}
}

func TestValidateBridgeServices_NonBridgeRuntimeIgnored(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	// A host service with no bridge fields should not trigger bridge validation
	if err := os.WriteFile(yamlPath, []byte(`
name: normal-test
services:
  - name: api
    runtime: host
    command: ["go", "run", "./cmd/api"]
    port: 8080
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(yamlPath, "")
	if err != nil {
		t.Fatalf("non-bridge service should not trigger bridge validation: %v", err)
	}
	if cfg.Services[0].BridgeTarget != nil || cfg.Services[0].BridgeIntercept != nil {
		t.Error("non-bridge service should have nil bridge fields")
	}
}
