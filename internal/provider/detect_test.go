package provider

import (
	"testing"
)

func TestDetect_ReturnsResults(t *testing.T) {
	// Detect() scans the real system. We can't control what's installed,
	// but we can verify the function doesn't panic and returns a valid slice.
	detected := Detect()
	if detected == nil {
		t.Fatal("Detect() returned nil, expected empty slice or results")
	}

	// Verify each result has required fields populated
	for _, d := range detected {
		if d.Name == "" {
			t.Error("detected provider has empty Name")
		}
		if d.Binary == "" {
			t.Error("detected provider has empty Binary")
		}
	}
}

func TestDetect_KnownProviderNames(t *testing.T) {
	detected := Detect()
	validNames := map[string]bool{
		"podman": true, "lima": true, "colima": true,
		"docker": true, "orbstack": true,
	}
	for _, d := range detected {
		if !validNames[d.Name] {
			t.Errorf("Detect() returned unknown provider name %q", d.Name)
		}
	}
}

func TestResolve_ExplicitPodman(t *testing.T) {
	vm, rt, err := Resolve("podman")
	if err != nil {
		t.Fatalf("Resolve(podman) error: %v", err)
	}
	if vm.Name() != "podman" {
		t.Errorf("expected podman VM, got %s", vm.Name())
	}
	if rt.Name() != "podman" {
		t.Errorf("expected podman runtime, got %s", rt.Name())
	}
	if !rt.SupportsCheckpoint() {
		t.Error("podman runtime should support checkpoint")
	}
}

func TestResolve_ExplicitLima(t *testing.T) {
	vm, rt, err := Resolve("lima")
	if err != nil {
		t.Fatalf("Resolve(lima) error: %v", err)
	}
	if vm.Name() != "lima" {
		t.Errorf("expected lima VM, got %s", vm.Name())
	}
	if rt.Name() != "nerdctl" {
		t.Errorf("expected nerdctl runtime for lima, got %s", rt.Name())
	}
	if rt.SupportsCheckpoint() {
		t.Error("nerdctl runtime should not support checkpoint")
	}
}

func TestResolve_ExplicitColima(t *testing.T) {
	vm, rt, err := Resolve("colima")
	if err != nil {
		t.Fatalf("Resolve(colima) error: %v", err)
	}
	if vm.Name() != "colima" {
		t.Errorf("expected colima VM, got %s", vm.Name())
	}
	if rt.Name() != "nerdctl" {
		t.Errorf("expected nerdctl runtime for colima, got %s", rt.Name())
	}
}

func TestResolve_ExplicitDocker(t *testing.T) {
	vm, rt, err := Resolve("docker")
	if err != nil {
		t.Fatalf("Resolve(docker) error: %v", err)
	}
	if vm.Name() != "docker" {
		t.Errorf("expected docker VM, got %s", vm.Name())
	}
	if rt.Name() != "docker" {
		t.Errorf("expected docker runtime, got %s", rt.Name())
	}
	if rt.SupportsCheckpoint() {
		t.Error("docker runtime should not support checkpoint")
	}
}

func TestResolve_UnknownProvider(t *testing.T) {
	_, _, err := Resolve("nonexistent-backend")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestMultipleProvidersError_Message(t *testing.T) {
	err := &MultipleProvidersError{
		Available: []DetectedProvider{
			{Name: "podman"}, {Name: "lima"},
		},
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
	if !contains(msg, "podman") || !contains(msg, "lima") {
		t.Errorf("error message should mention detected providers, got: %s", msg)
	}
}

func TestBuildProvider_AllNames(t *testing.T) {
	names := []string{"podman", "docker", "orbstack", "lima", "colima"}
	for _, name := range names {
		vm, rt, err := buildProvider(name)
		if err != nil {
			t.Errorf("buildProvider(%q) error: %v", name, err)
			continue
		}
		if vm == nil {
			t.Errorf("buildProvider(%q) returned nil VMProvider", name)
		}
		if rt == nil {
			t.Errorf("buildProvider(%q) returned nil ContainerRuntime", name)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
