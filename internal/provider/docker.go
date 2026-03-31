package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// DockerProvider implements VMProvider using Docker Desktop / OrbStack.
// OrbStack is automatically used when it is installed, since it replaces
// the `docker` CLI transparently.
type DockerProvider struct{}

func (d *DockerProvider) Name() string { return "docker" }

func (d *DockerProvider) Init(name string) error {
	// Docker Desktop / OrbStack creates the VM implicitly on first use.
	// We create a lightweight validation container to confirm the daemon is ready.
	var stderr bytes.Buffer
	cmd := exec.Command("docker", "info")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon check failed — is Docker Desktop or OrbStack running?\n%s", stderr.String())
	}
	return nil
}

func (d *DockerProvider) Start(_ string) error {
	// Docker Desktop / OrbStack manages its own VM lifecycle.
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not running — start Docker Desktop or OrbStack first")
	}
	return nil
}

func (d *DockerProvider) StopAll() error {
	// Stop all running containers managed by devx
	cmd := exec.Command("docker", "ps", "-q", "--filter", "label=managed-by=devx")
	out, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil
	}
	ids := strings.Fields(strings.TrimSpace(string(out)))
	args := append([]string{"stop"}, ids...)
	return exec.Command("docker", args...).Run()
}

func (d *DockerProvider) Remove(name string) error {
	_ = exec.Command("docker", "rm", "-f", name).Run()
	return nil
}

func (d *DockerProvider) SetDefault(_ string) error {
	// Docker uses a single default context; no multi-machine switching needed.
	return nil
}

func (d *DockerProvider) SSH(machineName, command string) (string, error) {
	// Docker Desktop / OrbStack: exec into a privileged container to run host commands.
	// OrbStack exposes `orb` for direct shell access; Docker Desktop uses nsenter via a privileged container.
	if orbPath, err := exec.LookPath("orb"); err == nil && orbPath != "" {
		cmd := exec.Command("orb", "sudo", "sh", "-c", command)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return strings.TrimSpace(string(out)), fmt.Errorf("orb exec (%s): %w", command, err)
		}
		return strings.TrimSpace(string(out)), nil
	}

	// Fallback: Docker Desktop — nsenter into the VM via a privileged container
	cmd := exec.Command("docker", "run", "--rm", "--privileged", "--pid=host",
		"alpine:latest", "nsenter", "-t", "1", "-m", "-u", "-n", "-i", "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("docker nsenter (%s): %w", command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *DockerProvider) IsRunning(_ string) bool {
	err := exec.Command("docker", "info").Run()
	return err == nil
}

type dockerInfo struct {
	ServerVersion string `json:"ServerVersion"`
}

func (d *DockerProvider) Inspect(_ string) (*MachineInfo, error) {
	out, err := exec.Command("docker", "info", "--format", "{{json .}}").Output()
	if err != nil {
		return nil, fmt.Errorf("docker info: %w", err)
	}
	var info dockerInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parsing docker info: %w", err)
	}
	state := "running"
	if info.ServerVersion == "" {
		state = "stopped"
	}
	return &MachineInfo{Name: "docker-desktop", State: state}, nil
}
