package tailscale

import (
	"fmt"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/podman"
)

// WaitForDaemon polls until the tailscaled container is running inside the VM.
func WaitForDaemon(machineName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := podman.SSH(machineName, "sudo podman inspect tailscaled > /dev/null 2>&1")
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("tailscaled did not start within %v — check: podman machine ssh %s 'sudo journalctl -u tailscaled -e --no-pager'", timeout, machineName)
}

// Up runs tailscale up inside the VM and returns the auth URL if one is
// printed. When already authenticated, returns an empty string.
func Up(machineName, hostname string) (string, error) {
	out, err := podman.SSH(machineName,
		fmt.Sprintf("sudo podman exec tailscaled timeout 5 tailscale up --accept-routes --hostname=%s 2>&1 || true", hostname))

	authURL := ExtractAuthURL(out)
	if err != nil && authURL == "" {
		return "", fmt.Errorf("tailscale up: %w\nOutput: %s", err, out)
	}
	return authURL, nil
}

// Status returns a brief status string from tailscale inside the VM.
func Status(machineName string) string {
	out, err := podman.SSH(machineName,
		"sudo podman exec tailscaled tailscale status --self 2>/dev/null | head -1")
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
