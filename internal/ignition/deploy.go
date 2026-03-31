package ignition

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/podman"
)

type ignConfig struct {
	Storage struct {
		Files []struct {
			Path     string `json:"path"`
			Mode     int    `json:"mode"`
			Contents struct {
				Source      string `json:"source"`
				Compression string `json:"compression"`
			} `json:"contents"`
		} `json:"files"`
	} `json:"storage"`
	Systemd struct {
		Units []struct {
			Name     string `json:"name"`
			Enabled  bool   `json:"enabled"`
			Contents string `json:"contents"`
		} `json:"units"`
	} `json:"systemd"`
}

// Deploy applies the Ignition config by SSH-ing into the VM rather than
// using --ignition-path, dodging Podman 5's networking breakage.
func Deploy(ignPath, machineName string) error {
	data, err := os.ReadFile(ignPath)
	if err != nil {
		return fmt.Errorf("reading ign path %s: %w", ignPath, err)
	}
	
	var conf ignConfig
	if err := json.Unmarshal(data, &conf); err != nil {
		return fmt.Errorf("unmarshaling ignition json: %w", err)
	}

	// Make necessary paths First
	podman.SSH(machineName, "sudo mkdir -p /etc/cloudflared /var/lib/tailscale /etc/sysctl.d")

	for _, f := range conf.Storage.Files {
		content := f.Contents.Source
		if strings.HasPrefix(content, "data:;base64,") {
			b64str := strings.TrimPrefix(content, "data:;base64,")
			decoded, err := base64.StdEncoding.DecodeString(b64str)
			if err == nil {
				if f.Contents.Compression == "gzip" {
					gz, err := gzip.NewReader(bytes.NewReader(decoded))
					if err == nil {
						raw, _ := io.ReadAll(gz)
						gz.Close()
						decoded = raw
					}
				}
				content = string(decoded)
			}
		} else if strings.HasPrefix(content, "data:,") {
			content = strings.TrimPrefix(content, "data:,")
			decoded, err := url.PathUnescape(content)
			if err == nil {
				content = decoded
			}
		}
		
		script := fmt.Sprintf("sudo tee %s >/dev/null <<'EOF'\n%s\nEOF\nsudo chmod %o %s", f.Path, content, f.Mode, f.Path)
		if _, err := podman.SSH(machineName, script); err != nil {
			return fmt.Errorf("deploying file %s: %w", f.Path, err)
		}
	}
    // Re-evaluate sysctls
    podman.SSH(machineName, "sudo sysctl --system")

	var enabledUnits []string
	for _, u := range conf.Systemd.Units {
		path := fmt.Sprintf("/etc/systemd/system/%s", u.Name)
		script := fmt.Sprintf("sudo tee %s >/dev/null <<'EOF'\n%s\nEOF\n", path, strings.TrimSpace(u.Contents))
		if _, err := podman.SSH(machineName, script); err != nil {
			return fmt.Errorf("deploying unit %s: %w", u.Name, err)
		}
		if u.Enabled {
			enabledUnits = append(enabledUnits, u.Name)
		}
	}
	
	if _, err := podman.SSH(machineName, "sudo systemctl daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	
	for _, u := range enabledUnits {
		if _, err := podman.SSH(machineName, "sudo systemctl enable --now "+u); err != nil {
			return fmt.Errorf("systemctl enable --now %s: %w", u, err)
		}
		podman.SSH(machineName, "sudo systemctl restart "+u)
	}

	return nil
}
