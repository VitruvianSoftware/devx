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

