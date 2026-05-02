// Package provider defines the VMProvider interface that abstracts away
// the underlying virtualization backend (Podman Machine, Docker Desktop,
// OrbStack, Lima, Colima, etc.) so that devx networking and provisioning
// can run on top of whatever hypervisor the developer already has.
//
// The package uses a two-layer architecture:
//
//   - VMProvider handles VM lifecycle (create, start, stop, SSH).
//   - ContainerRuntime handles container execution within the VM
//     (run, stop, checkpoint).
//
// The Provider composite binds a VMProvider and ContainerRuntime together
// so that commands can access both layers through a single value.
package provider

import (
	"context"
	"os"
	"os/exec"
)

// MachineInfo is the backend-agnostic representation of a VM.
type MachineInfo struct {
	Name  string
	State string
}

// VMProvider is the contract every VM backend must fulfil.
// It handles the VM lifecycle: provisioning, starting, stopping, and SSH.
type VMProvider interface {
	// Name returns the human-readable provider name (e.g. "podman", "lima").
	Name() string

	// Init provisions a new VM with the given name.
	Init(name string) error

	// Start starts a named VM.
	Start(name string) error

	// StopAll stops all running VMs managed by this provider.
	StopAll() error

	// Remove force-removes a VM by name.
	Remove(name string) error

	// Sleep suspends the VM to save memory/battery.
	Sleep(name string) error

	// Resize modifies the VM's hardware limits (cpus, memory).
	Resize(name string, cpus int, memoryMB int) error

	// SetDefault sets the active connection context to the named VM.
	SetDefault(name string) error

	// SSH executes a shell command inside the named VM and returns stdout.
	SSH(machineName, command string) (string, error)

	// IsRunning checks if the named VM is in the "running" state.
	IsRunning(name string) bool

	// Inspect returns structured info about a machine.
	Inspect(name string) (*MachineInfo, error)
}

// ContainerRuntime executes container commands within the VM backend.
// Each VMProvider has a paired ContainerRuntime that knows how to invoke
// the correct CLI (podman, docker, nerdctl) for that backend.
type ContainerRuntime interface {
	// Name returns the runtime CLI name (e.g. "podman", "docker", "nerdctl").
	Name() string

	// RunInteractive executes a container with stdin/stdout/stderr attached.
	// This replaces direct exec.Command calls in cmd/shell.go so that each
	// backend can prepend the correct wrapper (e.g. "limactl shell <vm> nerdctl").
	RunInteractive(args ...string) error

	// Exec runs a container command and returns combined output.
	Exec(args ...string) (string, error)

	// CommandContext returns an *exec.Cmd to run a container command with context.
	CommandContext(ctx context.Context, args ...string) *exec.Cmd

	// SupportsCheckpoint reports whether CRIU checkpoint/restore is available.
	SupportsCheckpoint() bool
}

// Provider is the composite that binds a VM backend and its container runtime.
// Commands that need both VM lifecycle and container execution use this type.
type Provider struct {
	VM      VMProvider
	Runtime ContainerRuntime
}

// --- Default ContainerRuntime implementations ---

// PodmanRuntime executes containers via the native `podman` CLI.
type PodmanRuntime struct{}

func (r *PodmanRuntime) Name() string { return "podman" }

func (r *PodmanRuntime) RunInteractive(args ...string) error {
	cmd := exec.Command("podman", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *PodmanRuntime) Exec(args ...string) (string, error) {
	out, err := exec.Command("podman", args...).CombinedOutput()
	return string(out), err
}

func (r *PodmanRuntime) CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "podman", args...)
}

func (r *PodmanRuntime) SupportsCheckpoint() bool { return true }

// DockerRuntime executes containers via the native `docker` CLI.
type DockerRuntime struct{}

func (r *DockerRuntime) Name() string { return "docker" }

func (r *DockerRuntime) RunInteractive(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *DockerRuntime) Exec(args ...string) (string, error) {
	out, err := exec.Command("docker", args...).CombinedOutput()
	return string(out), err
}

func (r *DockerRuntime) CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", args...)
}

func (r *DockerRuntime) SupportsCheckpoint() bool { return false }

// NerdctlRuntime executes containers via `nerdctl` inside a Lima/Colima VM.
// Commands are proxied through the VM shell (e.g. "limactl shell <vm> nerdctl ...").
type NerdctlRuntime struct {
	// ShellCmd is the command prefix to enter the VM shell.
	// For Lima: []string{"limactl", "shell", "<vmname>"}
	// For Colima: []string{"colima", "ssh", "--profile", "<profile>", "--"}
	ShellCmd []string
}

func (r *NerdctlRuntime) Name() string { return "nerdctl" }

func (r *NerdctlRuntime) RunInteractive(args ...string) error {
	fullArgs := append(r.ShellCmd, append([]string{"nerdctl"}, args...)...)
	cmd := exec.Command(fullArgs[0], fullArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *NerdctlRuntime) Exec(args ...string) (string, error) {
	fullArgs := append(r.ShellCmd, append([]string{"nerdctl"}, args...)...)
	out, err := exec.Command(fullArgs[0], fullArgs[1:]...).CombinedOutput()
	return string(out), err
}

func (r *NerdctlRuntime) CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	fullArgs := append(r.ShellCmd, append([]string{"nerdctl"}, args...)...)
	return exec.CommandContext(ctx, fullArgs[0], fullArgs[1:]...)
}

func (r *NerdctlRuntime) SupportsCheckpoint() bool { return false }
