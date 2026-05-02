package provider

import (
	"fmt"
	"os/exec"
	"strings"
)

// DetectedProvider holds information about a VM backend found on the system.
type DetectedProvider struct {
	Name    string // "podman", "lima", "colima", "docker", "orbstack"
	Binary  string // path to the binary
	Version string // detected version string
}

// providerBinary maps provider names to their expected binary on $PATH.
var providerBinary = map[string]string{
	"podman":   "podman",
	"lima":     "limactl",
	"colima":   "colima",
	"docker":   "docker",
	"orbstack": "orb",
}

// detectOrder is the preferred order for auto-detection.
// Podman is first because it is the historically default provider for devx.
var detectOrder = []string{"podman", "lima", "colima", "docker", "orbstack"}

// Detect scans the system for available VM backends and returns them
// in preference order. Only backends whose CLI binary is found on $PATH
// are included.
func Detect() []DetectedProvider {
	var found []DetectedProvider
	for _, name := range detectOrder {
		binary := providerBinary[name]
		path, err := exec.LookPath(binary)
		if err != nil {
			continue
		}

		dp := DetectedProvider{
			Name:   name,
			Binary: path,
		}

		// Best-effort version detection
		dp.Version = detectVersion(binary)

		found = append(found, dp)
	}
	return found
}

// Resolve picks a provider by name. If name is "auto", it calls Detect()
// and either auto-selects (if exactly one is available) or returns an error
// asking the caller to pick interactively.
//
// The interactive prompt itself lives in the cmd layer (not here) so that
// this package stays independent of TUI libraries.
func Resolve(name string) (VMProvider, ContainerRuntime, error) {
	if name == "" || name == "auto" {
		return resolveAuto()
	}
	return resolveExplicit(name)
}

func resolveAuto() (VMProvider, ContainerRuntime, error) {
	detected := Detect()
	if len(detected) == 0 {
		return nil, nil, fmt.Errorf(
			"no supported VM backend found.\n" +
				"Install one of: podman, lima, colima, docker, or orbstack.\n" +
				"  brew install podman    # recommended\n" +
				"  brew install lima      # lightweight alternative\n" +
				"  brew install colima    # Lima with batteries included\n" +
				"Run 'devx doctor' for full prerequisite details.")
	}
	if len(detected) == 1 {
		return buildProvider(detected[0].Name)
	}

	// Multiple providers detected — return a sentinel error so the cmd
	// layer can display an interactive picker.
	names := make([]string, len(detected))
	for i, d := range detected {
		names[i] = d.Name
	}
	return nil, nil, &MultipleProvidersError{Available: detected}
}

func resolveExplicit(name string) (VMProvider, ContainerRuntime, error) {
	binary, ok := providerBinary[name]
	if !ok {
		return nil, nil, fmt.Errorf("unknown provider %q — supported: podman, lima, colima, docker, orbstack", name)
	}

	if _, err := exec.LookPath(binary); err != nil {
		return nil, nil, fmt.Errorf("provider %q selected but %q not found on $PATH.\nInstall with: brew install %s",
			name, binary, installName(name))
	}

	return buildProvider(name)
}

// buildProvider creates the VMProvider + ContainerRuntime pair for a given name.
func buildProvider(name string) (VMProvider, ContainerRuntime, error) {
	switch name {
	case "podman":
		return &PodmanProvider{}, &PodmanRuntime{}, nil
	case "docker", "orbstack":
		return &DockerProvider{}, &DockerRuntime{}, nil
	case "lima":
		return &LimaProvider{}, &NerdctlRuntime{
			ShellCmd: []string{"limactl", "shell", "default"},
		}, nil
	case "colima":
		return &ColimaProvider{}, &NerdctlRuntime{
			ShellCmd: []string{"colima", "ssh", "--"},
		}, nil
	default:
		return nil, nil, fmt.Errorf("unknown provider %q", name)
	}
}

// MultipleProvidersError is returned when auto-detection finds more than one
// provider and the caller needs to prompt the user to choose.
type MultipleProvidersError struct {
	Available []DetectedProvider
}

func (e *MultipleProvidersError) Error() string {
	names := make([]string, len(e.Available))
	for i, d := range e.Available {
		names[i] = d.Name
	}
	return fmt.Sprintf("multiple VM backends detected: %s — set --provider or configure in ~/.devx/config.yaml",
		strings.Join(names, ", "))
}

// --- helpers ---

func detectVersion(binary string) string {
	// Try --version first, then version (no dashes)
	for _, flag := range []string{"--version", "version"} {
		out, err := exec.Command(binary, flag).CombinedOutput()
		if err == nil {
			lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
			if len(lines) > 0 {
				return lines[0]
			}
		}
	}
	return ""
}

func installName(provider string) string {
	switch provider {
	case "orbstack":
		return "orbstack"
	case "lima":
		return "lima"
	default:
		return provider
	}
}
