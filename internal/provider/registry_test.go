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
