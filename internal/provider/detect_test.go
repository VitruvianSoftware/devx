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

package provider

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakePath points $PATH at a fresh directory holding a stub executable for
// each given binary name, so Detect() and Resolve() see exactly those
// backends and nothing from the host. Each stub prints "<name> version 1.2.3"
// for any argument, which is what detectVersion() reads. Calling it with no
// names gives a PATH on which no backend is installed.
func fakePath(t *testing.T, binaries ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, b := range binaries {
		script := "#!/bin/sh\necho \"" + b + " version 1.2.3\"\n"
		if err := os.WriteFile(filepath.Join(dir, b), []byte(script), 0o755); err != nil {
			t.Fatalf("writing stub %s: %v", b, err)
		}
	}
	t.Setenv("PATH", dir)
	return dir
}

func TestDetect_NoneInstalledReturnsEmptySlice(t *testing.T) {
	fakePath(t)
	detected := Detect()
	if detected == nil {
		t.Fatal("Detect() returned nil, want an empty non-nil slice")
	}
	if len(detected) != 0 {
		t.Fatalf("Detect() = %v, want no providers", detected)
	}
}

func TestDetect_FindsInstalledInPreferenceOrder(t *testing.T) {
	// Deliberately not in preference order: Detect() must sort by detectOrder.
	dir := fakePath(t, "docker", "limactl", "podman")
	detected := Detect()

	var names []string
	for _, d := range detected {
		names = append(names, d.Name)
	}
	if want := []string{"podman", "lima", "docker"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("Detect() names = %v, want %v", names, want)
	}
	for _, d := range detected {
		bin := providerBinary[d.Name]
		if want := filepath.Join(dir, bin); d.Binary != want {
			t.Errorf("%s: Binary = %q, want %q", d.Name, d.Binary, want)
		}
		if want := bin + " version 1.2.3"; d.Version != want {
			t.Errorf("%s: Version = %q, want %q", d.Name, d.Version, want)
		}
	}
}

func TestResolve_AutoNoneInstalled(t *testing.T) {
	fakePath(t)
	_, _, err := Resolve("auto")
	if err == nil || !strings.Contains(err.Error(), "no supported VM backend found") {
		t.Fatalf("Resolve(auto) error = %v, want 'no supported VM backend found'", err)
	}
}

func TestResolve_AutoSingleInstalled(t *testing.T) {
	fakePath(t, "colima")
	vm, rt, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\") error: %v", err)
	}
	if vm.Name() != "colima" || rt.Name() != "nerdctl" {
		t.Fatalf("Resolve(\"\") = %s/%s, want colima/nerdctl", vm.Name(), rt.Name())
	}
}

func TestResolve_AutoMultipleInstalled(t *testing.T) {
	fakePath(t, "podman", "orb")
	_, _, err := Resolve("auto")
	var multi *MultipleProvidersError
	if !errors.As(err, &multi) {
		t.Fatalf("Resolve(auto) error = %v, want *MultipleProvidersError", err)
	}
	if len(multi.Available) != 2 || multi.Available[0].Name != "podman" || multi.Available[1].Name != "orbstack" {
		t.Fatalf("Available = %v, want [podman orbstack]", multi.Available)
	}
}

func TestResolve_ExplicitNotInstalled(t *testing.T) {
	fakePath(t)
	_, _, err := Resolve("podman")
	if err == nil || !strings.Contains(err.Error(), "not found on $PATH") {
		t.Fatalf("Resolve(podman) error = %v, want 'not found on $PATH'", err)
	}
}

func TestResolve_Explicit(t *testing.T) {
	tests := []struct {
		name, binary, wantVM, wantRuntime string
		wantCheckpoint                    bool
	}{
		{"podman", "podman", "podman", "podman", true},
		{"lima", "limactl", "lima", "nerdctl", false},
		{"colima", "colima", "colima", "nerdctl", false},
		{"docker", "docker", "docker", "docker", false},
		{"orbstack", "orb", "docker", "docker", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakePath(t, tt.binary)
			vm, rt, err := Resolve(tt.name)
			if err != nil {
				t.Fatalf("Resolve(%s) error: %v", tt.name, err)
			}
			if vm.Name() != tt.wantVM {
				t.Errorf("VM = %s, want %s", vm.Name(), tt.wantVM)
			}
			if rt.Name() != tt.wantRuntime {
				t.Errorf("runtime = %s, want %s", rt.Name(), tt.wantRuntime)
			}
			if rt.SupportsCheckpoint() != tt.wantCheckpoint {
				t.Errorf("SupportsCheckpoint = %v, want %v", rt.SupportsCheckpoint(), tt.wantCheckpoint)
			}
		})
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
	if !strings.Contains(msg, "podman") || !strings.Contains(msg, "lima") {
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
