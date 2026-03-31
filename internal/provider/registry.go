package provider

import "fmt"

// Get returns a VMProvider for the given name.
// Supported values: "podman" (default), "docker", "orbstack".
func Get(name string) (VMProvider, error) {
	switch name {
	case "", "podman":
		return &PodmanProvider{}, nil
	case "docker", "orbstack":
		return &DockerProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q — supported: podman, docker, orbstack", name)
	}
}
