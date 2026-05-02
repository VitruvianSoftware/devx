package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadLocal_MissingFile(t *testing.T) {
	t.Setenv("DEVX_CONFIG_DIR", t.TempDir())

	cfg, err := LoadLocal()
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cfg.Provider != "" {
		t.Errorf("expected empty provider, got %s", cfg.Provider)
	}
}

func TestSaveAndLoadLocal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEVX_CONFIG_DIR", dir)

	input := &LocalConfig{Provider: "lima"}
	if err := SaveLocal(input); err != nil {
		t.Fatalf("SaveLocal error: %v", err)
	}

	// Verify the file exists
	path := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config.yaml was not created")
	}

	// Load it back
	loaded, err := LoadLocal()
	if err != nil {
		t.Fatalf("LoadLocal error: %v", err)
	}
	if loaded.Provider != "lima" {
		t.Errorf("expected provider=lima, got %s", loaded.Provider)
	}
}

func TestSaveLocal_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	t.Setenv("DEVX_CONFIG_DIR", dir)

	if err := SaveLocal(&LocalConfig{Provider: "podman"}); err != nil {
		t.Fatalf("SaveLocal error: %v", err)
	}

	// Directory should have been created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("nested directory was not created")
	}
}

func TestLoadLocal_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEVX_CONFIG_DIR", dir)

	// Write invalid YAML
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("{{not yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadLocal()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLocalConfigDir_Default(t *testing.T) {
	t.Setenv("DEVX_CONFIG_DIR", "")
	dir := LocalConfigDir()
	if dir == "" {
		t.Fatal("expected non-empty config dir")
	}
}

func TestLocalConfigDir_Override(t *testing.T) {
	t.Setenv("DEVX_CONFIG_DIR", "/tmp/test-devx")
	dir := LocalConfigDir()
	if dir != "/tmp/test-devx" {
		t.Errorf("expected /tmp/test-devx, got %s", dir)
	}
}
