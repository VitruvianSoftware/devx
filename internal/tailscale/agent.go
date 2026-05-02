package tailscale

import (
	"fmt"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/podman"
)

// SSHFunc is a function that executes a command on a remote machine.
type SSHFunc func(machineName, command string) (string, error)

// WaitForDaemon polls until the tailscaled container is running inside the VM.
// Uses the legacy podman.SSH call for backward compatibility.
func WaitForDaemon(machineName string, timeout time.Duration) error {
	return WaitForDaemonWithSSH(machineName, "podman", timeout, podman.SSH)
}

// WaitForDaemonWithSSH polls until the tailscaled container is running,
// using the provided SSH function for provider-agnostic execution.
func WaitForDaemonWithSSH(machineName, runtime string, timeout time.Duration, sshFn SSHFunc) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := sshFn(machineName, fmt.Sprintf("sudo %s inspect tailscaled > /dev/null 2>&1", runtime))
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("tailscaled did not start within %v — check: devx exec %s logs tailscaled", timeout, runtime)
}

// Up runs tailscale up inside the VM and returns the auth URL if one is
// printed. Uses the legacy podman.SSH call.
func Up(machineName, hostname string) (string, error) {
	return UpWithSSH(machineName, "podman", hostname, podman.SSH)
}

// UpWithSSH runs tailscale up using the provided SSH function.
func UpWithSSH(machineName, runtime, hostname string, sshFn SSHFunc) (string, error) {
	out, err := sshFn(machineName,
		fmt.Sprintf("sudo %s exec tailscaled timeout 5 tailscale up --accept-routes --hostname=%s 2>&1 || true", runtime, hostname))

	authURL := ExtractAuthURL(out)
	if err != nil && authURL == "" {
		return "", fmt.Errorf("tailscale up: %w\nOutput: %s", err, out)
	}
	return authURL, nil
}

// Status returns a brief status string from tailscale inside the VM.
// Uses the legacy podman.SSH call.
func Status(machineName string) string {
	return StatusWithSSH(machineName, "podman", podman.SSH)
}

// StatusWithSSH returns a brief status string using the provided SSH function.
func StatusWithSSH(machineName, runtime string, sshFn SSHFunc) string {
	out, err := sshFn(machineName,
		fmt.Sprintf("sudo %s exec tailscaled tailscale status --self 2>/dev/null | head -1", runtime))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}

// ExtractAuthURL scans output for a Tailscale login URL and returns it.
// Exported for testing.
func ExtractAuthURL(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "https://login.tailscale.com") {
			return trimmed
		}
	}
	return ""
}
