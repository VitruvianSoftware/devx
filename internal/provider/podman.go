package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// PodmanProvider implements VMProvider using Podman Machine.
type PodmanProvider struct{}

func (p *PodmanProvider) Name() string { return "podman" }

func (p *PodmanProvider) Init(name string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("podman", "machine", "init", "--rootful", name)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman machine init: %w\n%s", err, stderr.String())
	}
	return nil
}

func (p *PodmanProvider) Start(name string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("podman", "machine", "start", name)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman machine start: %w\n%s", err, stderr.String())
	}
	return nil
}

func (p *PodmanProvider) StopAll() error {
	cmd := exec.Command("podman", "machine", "stop")
	_ = cmd.Run()
	return nil
}

func (p *PodmanProvider) Sleep(name string) error {
	cmd := exec.Command("podman", "machine", "stop", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("podman machine stop: %w\n%s", err, string(out))
	}
	return nil
}

func (p *PodmanProvider) Resize(name string, cpus int, memoryMB int) error {
	wasRunning := p.IsRunning(name)
	if wasRunning {
		fmt.Println("Stopping machine to resize...")
		_ = p.Sleep(name)
	}

	args := []string{"machine", "set"}
	if cpus > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", cpus))
	}
	if memoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%d", memoryMB))
	}
	args = append(args, name)

	cmd := exec.Command("podman", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("podman machine set: %w\n%s", err, string(out))
	}

	if wasRunning {
		fmt.Println("Restarting machine...")
		return p.Start(name)
	}
	return nil
}

func (p *PodmanProvider) Remove(name string) error {
	cmd := exec.Command("podman", "machine", "rm", "-f", name)
	_ = cmd.Run()
	return nil
}

func (p *PodmanProvider) SetDefault(name string) error {
	return exec.Command("podman", "system", "connection", "default", name).Run()
}

func (p *PodmanProvider) SSH(machineName, command string) (string, error) {
	cmd := exec.Command("podman", "machine", "ssh", machineName, command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("podman machine ssh (%s): %w", command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *PodmanProvider) IsRunning(name string) bool {
	out, err := exec.Command("podman", "machine", "inspect",
		"--format", "{{.State}}", name).Output()
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(string(out)), "running")
}

type podmanMachineInfo struct {
	Name  string `json:"Name"`
	State string `json:"State"`
}

func (p *PodmanProvider) Inspect(name string) (*MachineInfo, error) {
	out, err := exec.Command("podman", "machine", "inspect", "--format", "json", name).Output()
	if err != nil {
		return nil, fmt.Errorf("podman machine inspect: %w", err)
	}
	var infos []podmanMachineInfo
	if err := json.Unmarshal(out, &infos); err != nil {
		return nil, fmt.Errorf("parsing machine info: %w", err)
	}
	if len(infos) == 0 {
		return nil, fmt.Errorf("machine %q not found", name)
	}
	return &MachineInfo{Name: infos[0].Name, State: infos[0].State}, nil
}
