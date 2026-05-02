package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// LimaProvider implements VMProvider using Lima (limactl).
// Lima creates lightweight Linux VMs using macOS Virtualization.framework
// (or QEMU) with VirtioFS file sharing.
type LimaProvider struct{}

func (l *LimaProvider) Name() string { return "lima" }

func (l *LimaProvider) Init(name string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("limactl", "create",
		"--name="+name,
		"--vm-type=vz",
		"--mount-type=virtiofs",
		"--mount-writable",
		"--tty=false",
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("limactl create: %w\n%s", err, stderr.String())
	}
	return nil
}

func (l *LimaProvider) Start(name string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("limactl", "start", name)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("limactl start: %w\n%s", err, stderr.String())
	}
	return nil
}

func (l *LimaProvider) StopAll() error {
	cmd := exec.Command("limactl", "stop", "--all")
	_ = cmd.Run()
	return nil
}

func (l *LimaProvider) Sleep(name string) error {
	cmd := exec.Command("limactl", "stop", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("limactl stop: %w\n%s", err, string(out))
	}
	return nil
}

func (l *LimaProvider) Resize(name string, cpus int, memoryMB int) error {
	wasRunning := l.IsRunning(name)
	if wasRunning {
		fmt.Println("Stopping Lima VM to resize...")
		_ = l.Sleep(name)
	}

	// Lima uses limactl edit to modify VM configuration.
	// We build a YAML override and pipe it into limactl edit.
	var overrides []string
	if cpus > 0 {
		overrides = append(overrides, fmt.Sprintf("cpus: %d", cpus))
	}
	if memoryMB > 0 {
		overrides = append(overrides, fmt.Sprintf("memory: %dMiB", memoryMB))
	}

	if len(overrides) > 0 {
		yaml := strings.Join(overrides, "\n") + "\n"
		cmd := exec.Command("limactl", "edit", "--set", yaml, name)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("limactl edit: %w\n%s", err, string(out))
		}
	}

	if wasRunning {
		fmt.Println("Restarting Lima VM...")
		return l.Start(name)
	}
	return nil
}

func (l *LimaProvider) Remove(name string) error {
	cmd := exec.Command("limactl", "delete", "--force", name)
	_ = cmd.Run()
	return nil
}

func (l *LimaProvider) SetDefault(_ string) error {
	// Lima doesn't have a "default machine" concept like Podman.
	// Each command targets a specific instance by name.
	return nil
}

func (l *LimaProvider) SSH(machineName, command string) (string, error) {
	cmd := exec.Command("limactl", "shell", machineName, "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("limactl shell (%s): %w", command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (l *LimaProvider) IsRunning(name string) bool {
	out, err := exec.Command("limactl", "list", "--json").Output()
	if err != nil {
		return false
	}

	// limactl list --json outputs one JSON object per line (JSONL)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var info limaInstanceInfo
		if err := json.Unmarshal([]byte(line), &info); err != nil {
			continue
		}
		if info.Name == name {
			return strings.EqualFold(info.Status, "running")
		}
	}
	return false
}

type limaInstanceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Arch   string `json:"arch"`
	CPUs   int    `json:"cpus"`
	Memory int64  `json:"memory"`
	Disk   int64  `json:"disk"`
}

func (l *LimaProvider) Inspect(name string) (*MachineInfo, error) {
	out, err := exec.Command("limactl", "list", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("limactl list: %w", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var info limaInstanceInfo
		if err := json.Unmarshal([]byte(line), &info); err != nil {
			continue
		}
		if info.Name == name {
			return &MachineInfo{Name: info.Name, State: info.Status}, nil
		}
	}
	return nil, fmt.Errorf("lima instance %q not found", name)
}
