package cloudflare

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Tunnel represents a Cloudflare tunnel entry from the API.
type Tunnel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// CheckLogin verifies that cloudflared has a valid certificate on disk.
func CheckLogin() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	certPath := home + "/.cloudflared/cert.pem"
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return fmt.Errorf("cloudflared not authenticated — run 'cloudflared login' first")
	}
	return nil
}

// EnsureTunnel returns the existing tunnel by name or creates a new one.
func EnsureTunnel(name string) (*Tunnel, error) {
	tunnels, err := listTunnels()
	if err != nil {
		return nil, err
	}

	for i := range tunnels {
		if tunnels[i].Name == name {
			return &tunnels[i], nil
		}
	}

	// Tunnel doesn't exist — create it
	out, err := exec.Command("cloudflared", "tunnel", "create", name).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("cloudflared tunnel create: %w\n%s", err, string(out))
	}

	// Re-list to get the generated UUID
	tunnels, err = listTunnels()
	if err != nil {
		return nil, err
	}
	for i := range tunnels {
		if tunnels[i].Name == name {
			return &tunnels[i], nil
		}
	}

	return nil, fmt.Errorf("tunnel %q not found after creation", name)
}

// GetToken returns the named-tunnel token for use with --token auth.
func GetToken(tunnelName string) (string, error) {
	out, err := exec.Command("cloudflared", "tunnel", "token", tunnelName).Output()
	if err != nil {
		return "", fmt.Errorf("cloudflared tunnel token: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RouteDNS idempotently creates a CNAME routing domain → tunnel.
// Failures are treated as non-fatal (DNS may already be routed).
func RouteDNS(tunnelName, domain string) error {
	out, err := exec.Command("cloudflared", "tunnel", "route", "dns", "-f", tunnelName, domain).CombinedOutput()
	if err != nil {
		// "already exists" responses are fine
		if strings.Contains(string(out), "already") {
			return nil
		}
		return fmt.Errorf("cloudflared route dns: %w\n%s", err, string(out))
	}
	return nil
}

// TunnelStatus returns a human-readable status string.
func TunnelStatus(name string) (string, error) {
	tunnels, err := listTunnels()
	if err != nil {
		return "", err
	}
	for _, t := range tunnels {
		if t.Name == name {
			return "active (" + t.ID[:8] + "...)", nil
		}
	}
	return "not found", nil
}

func listTunnels() ([]Tunnel, error) {
	out, err := exec.Command("cloudflared", "tunnel", "list", "--output", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("cloudflared tunnel list: %w", err)
	}
	var tunnels []Tunnel
	if err := json.Unmarshal(out, &tunnels); err != nil {
		return nil, fmt.Errorf("parsing tunnel list: %w", err)
	}
	return tunnels, nil
}

// CleanupExposedTunnels forcefully deletes any tunnels tracking to this user's exposed dev environment.
// Returns the number of tunnels cleaned up and any error.
func CleanupExposedTunnels(devName string) (int, error) {
	tunnels, err := listTunnels()
	if err != nil {
		return 0, err
	}

	count := 0
	prefix := fmt.Sprintf("devx-expose-%s-", devName)
	for _, t := range tunnels {
		if strings.HasPrefix(t.Name, prefix) {
			out, err := exec.Command("cloudflared", "tunnel", "delete", "-f", t.Name).CombinedOutput()
			if err != nil {
				return count, fmt.Errorf("failed deleting tunnel %s: %w\n%s", t.Name, err, string(out))
			}

			home, err := os.UserHomeDir()
			if err == nil {
				credFile := fmt.Sprintf("%s/.cloudflared/%s.json", home, t.ID)
				configFile := fmt.Sprintf("%s/.cloudflared/%s-config.yml", home, t.ID)
				os.Remove(credFile)
				os.Remove(configFile)
			}

			count++
		}
	}
	return count, nil
}

// ListExposedTunnels returns all active exposed tunnels for this environment.
func ListExposedTunnels(devName string) ([]Tunnel, error) {
	tunnels, err := listTunnels()
	if err != nil {
		return nil, err
	}
	var exposed []Tunnel
	prefix := fmt.Sprintf("devx-expose-%s-", devName)
	for _, t := range tunnels {
		if strings.HasPrefix(t.Name, prefix) {
			exposed = append(exposed, t)
		}
	}
	return exposed, nil
}

// WriteIngressConfig generates a temporary cloudflare tunnel ingress configuration file
// on disk. This is required for named tunnels since --url is ignored by cloudflared run
// unless an ingress config explicitly permits the hostname.
func WriteIngressConfig(tunnelID, fullDomain, targetPort string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	credFile := fmt.Sprintf("%s/.cloudflared/%s.json", home, tunnelID)
	configFile := fmt.Sprintf("%s/.cloudflared/%s-config.yml", home, tunnelID)

	configContent := fmt.Sprintf(`tunnel: %s
credentials-file: %s

ingress:
  - hostname: %s
    service: http://localhost:%s
  - service: http_status:404
`, tunnelID, credFile, fullDomain, targetPort)

	err = os.WriteFile(configFile, []byte(configContent), 0644)
	return configFile, err
}

// IngressEntry maps a domain to a port.
type IngressEntry struct {
	Hostname   string
	TargetPort string
}

// WriteMultiIngressConfig writes a configuration file with multiple routing destinations.
func WriteMultiIngressConfig(tunnelID string, entries []IngressEntry) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	credFile := fmt.Sprintf("%s/.cloudflared/%s.json", home, tunnelID)
	configFile := fmt.Sprintf("%s/.cloudflared/%s-config.yml", home, tunnelID)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("tunnel: %s\ncredentials-file: %s\n\ningress:\n", tunnelID, credFile))

	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("  - hostname: %s\n    service: http://localhost:%s\n", entry.Hostname, entry.TargetPort))
	}
	sb.WriteString("  - service: http_status:404\n")

	err = os.WriteFile(configFile, []byte(sb.String()), 0644)
	return configFile, err
}
