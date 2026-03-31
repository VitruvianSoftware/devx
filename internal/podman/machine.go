package podman

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// MachineInfo represents the JSON output of podman machine inspect.
type MachineInfo struct {
	Name  string `json:"Name"`
	State string `json:"State"`
}

// StopAll stops all running Podman machines (Podman supports one active VM).
func StopAll() error {
	cmd := exec.Command("podman", "machine", "stop")
	_ = cmd.Run() // non-fatal if nothing is running
	return nil
}

// Remove force-removes a Podman machine by name. Non-fatal if not found.
func Remove(name string) error {
	cmd := exec.Command("podman", "machine", "rm", "-f", name)
	_ = cmd.Run()
	return nil
}

// Init provisions a new Podman machine.
func Init(name string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("podman", "machine", "init",
		"--rootful",
		name,
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman machine init: %w\n%s", err, stderr.String())
	}
	return nil
}

// Start starts a named Podman machine.
func Start(name string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("podman", "machine", "start", name)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman machine start: %w\n%s", err, stderr.String())
	}
	return nil
}

// SetDefault sets the active Podman connection to the named machine.
func SetDefault(name string) error {
	return exec.Command("podman", "system", "connection", "default", name).Run()
}

// SSH executes a shell command inside the named Podman machine and returns stdout.
func SSH(machineName, command string) (string, error) {
	cmd := exec.Command("podman", "machine", "ssh", machineName, command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("podman machine ssh (%s): %w", command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsRunning checks if the named machine is in the "running" state.
func IsRunning(name string) bool {
	out, err := exec.Command("podman", "machine", "inspect",
		"--format", "{{.State}}", name).Output()
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(out)), "running")
}

// Inspect returns structured info about a machine.
func Inspect(name string) (*MachineInfo, error) {
	out, err := exec.Command("podman", "machine", "inspect", "--format", "json", name).Output()
	if err != nil {
		return nil, fmt.Errorf("podman machine inspect: %w", err)
	}
	// inspect returns a JSON array with one element
	var infos []MachineInfo
	if err := json.Unmarshal(out, &infos); err != nil {
		return nil, fmt.Errorf("parsing machine info: %w", err)
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("machine %q not found", name)
	}
	return &infos[0], nil
}
