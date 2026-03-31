package devcontainer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStandard(t *testing.T) {
	dir := t.TempDir()
	dcDir := filepath.Join(dir, ".devcontainer")
	if err := os.MkdirAll(dcDir, 0755); err != nil {
		t.Fatalf("creating dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dcDir, "devcontainer.json"), []byte(`{
		"name": "Go Dev",
		"image": "mcr.microsoft.com/devcontainers/go:1.22",
		"remoteUser": "vscode",
		"postCreateCommand": "go mod tidy",
		"containerEnv": {"GOPATH": "/home/vscode/go"},
		"forwardPorts": [8080, 3000]
	}`), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	cfg, path, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "Go Dev" {
		t.Errorf("expected name 'Go Dev', got %q", cfg.Name)
	}
	if cfg.Image != "mcr.microsoft.com/devcontainers/go:1.22" {
		t.Errorf("unexpected image: %s", cfg.Image)
	}
	if cfg.RemoteUser != "vscode" {
		t.Errorf("unexpected remoteUser: %s", cfg.RemoteUser)
	}
	if cfg.PostCreateCmd() != "go mod tidy" {
		t.Errorf("unexpected postCreateCommand: %s", cfg.PostCreateCmd())
	}
	if len(cfg.ForwardPorts) != 2 {
		t.Errorf("expected 2 forwarded ports, got %d", len(cfg.ForwardPorts))
	}
	if cfg.ContainerEnv["GOPATH"] != "/home/vscode/go" {
		t.Errorf("unexpected env: %v", cfg.ContainerEnv)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %s", path)
	}
}

func TestLoadRootLevel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".devcontainer.json"), []byte(`{
		"name": "Root",
		"image": "ubuntu:22.04"
	}`), 0644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	cfg, _, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "Root" {
		t.Errorf("expected name 'Root', got %q", cfg.Name)
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, _, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing devcontainer.json")
	}
}

func TestPostCreateCmdArray(t *testing.T) {
	cfg := &Config{
		PostCreateCommand: []interface{}{"npm install", "npm run build"},
	}
	got := cfg.PostCreateCmd()
	expected := "npm install && npm run build"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}
