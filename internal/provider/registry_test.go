package provider

import "testing"

func TestGetPodmanDefault(t *testing.T) {
	p, err := Get("")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Name() != "podman" {
		t.Fatalf("expected podman, got %s", p.Name())
	}
}

func TestGetPodmanExplicit(t *testing.T) {
	p, err := Get("podman")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Name() != "podman" {
		t.Fatalf("expected podman, got %s", p.Name())
	}
}

func TestGetDocker(t *testing.T) {
	p, err := Get("docker")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Name() != "docker" {
		t.Fatalf("expected docker, got %s", p.Name())
	}
}

func TestGetOrbStack(t *testing.T) {
	p, err := Get("orbstack")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Name() != "docker" {
		t.Fatalf("expected docker (orbstack alias), got %s", p.Name())
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := Get("unknown-backend")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
