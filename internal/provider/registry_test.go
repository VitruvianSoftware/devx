package provider

import (
	"strings"
	"testing"
)

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

func TestGetLima(t *testing.T) {
	p, err := Get("lima")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Name() != "lima" {
		t.Fatalf("expected lima, got %s", p.Name())
	}
}

func TestGetColima(t *testing.T) {
	p, err := Get("colima")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p.Name() != "colima" {
		t.Fatalf("expected colima, got %s", p.Name())
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := Get("unknown-backend")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestGetProvider_Explicit(t *testing.T) {
	prov, err := GetProvider("podman")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if prov.VM.Name() != "podman" {
		t.Fatalf("expected podman VM, got %s", prov.VM.Name())
	}
	if prov.Runtime.Name() != "podman" {
		t.Fatalf("expected podman runtime, got %s", prov.Runtime.Name())
	}
}

func TestGetProvider_Lima(t *testing.T) {
	prov, err := GetProvider("lima")
	if err != nil {
		if strings.Contains(err.Error(), "not found on $PATH") {
			t.Skipf("Skipping test: %v", err)
		}
		t.Fatalf("expected no error, got %v", err)
	}
	if prov.VM.Name() != "lima" {
		t.Fatalf("expected lima VM, got %s", prov.VM.Name())
	}
	if prov.Runtime.Name() != "nerdctl" {
		t.Fatalf("expected nerdctl runtime, got %s", prov.Runtime.Name())
	}
}

func TestGetProvider_Unknown(t *testing.T) {
	_, err := GetProvider("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
