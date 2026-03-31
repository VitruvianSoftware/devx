// Package provider defines the VMProvider interface that abstracts away
// the underlying virtualization backend (Podman Machine, Docker Desktop,
// OrbStack, etc.) so that devx networking and provisioning can run on top
// of whatever hypervisor the developer already has.
package provider

// MachineInfo is the backend-agnostic representation of a VM.
type MachineInfo struct {
	Name  string
	State string
}

// VMProvider is the contract every backend must fulfil.
type VMProvider interface {
	// Name returns the human-readable provider name (e.g. "podman", "docker").
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
