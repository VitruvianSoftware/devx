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
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ColimaProvider implements VMProvider using Colima, which wraps Lima
// with a simpler CLI and built-in container runtime management.
// Colima is treated as a "flavor" of Lima — architecturally it's the same
// VM engine, but with different CLI ergonomics (profiles instead of names).
type ColimaProvider struct{}

func (c *ColimaProvider) Name() string { return "colima" }

func (c *ColimaProvider) Init(name string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("colima", "start",
		"--profile", name,
		"--vm-type=vz",
		"--mount-type=virtiofs",
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("colima start: %w\n%s", err, stderr.String())
	}
	return nil
}

func (c *ColimaProvider) Start(name string) error {
	var stderr bytes.Buffer
	cmd := exec.Command("colima", "start", "--profile", name)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("colima start: %w\n%s", err, stderr.String())
	}
	return nil
}

func (c *ColimaProvider) StopAll() error {
	// Colima doesn't have a --all flag; we stop the default profile.
	// In practice, devx only manages a single colima profile.
	cmd := exec.Command("colima", "stop")
	_ = cmd.Run()
	return nil
}

func (c *ColimaProvider) Sleep(name string) error {
	cmd := exec.Command("colima", "stop", "--profile", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("colima stop: %w\n%s", err, string(out))
	}
	return nil
}

func (c *ColimaProvider) Resize(name string, cpus int, memoryMB int) error {
	wasRunning := c.IsRunning(name)
	if wasRunning {
		fmt.Println("Stopping Colima VM to resize...")
		_ = c.Sleep(name)
	}

	// Colima passes cpu/memory as start flags; restart with new values.
	args := []string{"start", "--profile", name}
	if cpus > 0 {
		args = append(args, "--cpu", fmt.Sprintf("%d", cpus))
	}
	if memoryMB > 0 {
		// Colima uses GiB for memory
		memGiB := memoryMB / 1024
		if memGiB < 1 {
			memGiB = 1
		}
		args = append(args, "--memory", fmt.Sprintf("%d", memGiB))
	}

	cmd := exec.Command("colima", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("colima resize: %w\n%s", err, string(out))
	}
	return nil
}

func (c *ColimaProvider) Remove(name string) error {
	cmd := exec.Command("colima", "delete", "--force", "--profile", name)
	_ = cmd.Run()
	return nil
}

func (c *ColimaProvider) SetDefault(_ string) error {
	// Colima uses profiles, not default connections.
	return nil
}

func (c *ColimaProvider) SSH(machineName, command string) (string, error) {
	cmd := exec.Command("colima", "ssh", "--profile", machineName, "--", "sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), fmt.Errorf("colima ssh (%s): %w", command, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *ColimaProvider) IsRunning(name string) bool {
	cmd := exec.Command("colima", "status", "--profile", name)
	return cmd.Run() == nil
}

func (c *ColimaProvider) Inspect(name string) (*MachineInfo, error) {
	state := "stopped"
	if c.IsRunning(name) {
		state = "running"
	}
	return &MachineInfo{Name: name, State: state}, nil
}
