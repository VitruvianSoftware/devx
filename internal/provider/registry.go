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

import "fmt"

// Get returns a VMProvider for the given name.
// Supported values: "podman" (default), "docker", "orbstack", "lima", "colima".
// An empty string or "auto" triggers auto-detection.
func Get(name string) (VMProvider, error) {
	switch name {
	case "podman":
		return &PodmanProvider{}, nil
	case "docker", "orbstack":
		return &DockerProvider{}, nil
	case "lima":
		return &LimaProvider{}, nil
	case "colima":
		return &ColimaProvider{}, nil
	case "", "auto":
		// Auto-detect: return the VMProvider from Resolve.
		vm, _, err := Resolve("auto")
		return vm, err
	default:
		return nil, fmt.Errorf("unknown provider %q — supported: podman, docker, orbstack, lima, colima", name)
	}
}

// GetProvider returns the full Provider composite (VM + Runtime) for a name.
// This is the preferred API for new code that needs container runtime access.
func GetProvider(name string) (*Provider, error) {
	if name == "" {
		name = "auto"
	}
	vm, rt, err := Resolve(name)
	if err != nil {
		return nil, err
	}
	return &Provider{VM: vm, Runtime: rt}, nil
}

