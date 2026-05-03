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

// SSHFunc is a function that executes a command on a remote machine via SSH.
type SSHFunc func(machineName, command string) (string, error)

// Deploy applies the Ignition config using the legacy podman.SSH call.
// Deprecated: Use DeployWithSSH for provider-agnostic deployments.
func Deploy(ignPath, machineName string) error {
	return DeployWithSSH(ignPath, machineName, podman.SSH)
}

// DeployWithSSH applies the Ignition config by executing commands on the
// VM through the provided SSH function. This supports any virtualization backend.
func DeployWithSSH(ignPath, machineName string, sshFn SSHFunc) error {
	data, err := os.ReadFile(ignPath)
	if err != nil {
		return fmt.Errorf("reading ign path %s: %w", ignPath, err)
	}

	var conf ignConfig
	if err := json.Unmarshal(data, &conf); err != nil {
		return fmt.Errorf("unmarshaling ignition json: %w", err)
	}

	// Make necessary paths First
	_, _ = sshFn(machineName, "sudo mkdir -p /etc/cloudflared /var/lib/tailscale /etc/sysctl.d")

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
						_ = gz.Close()
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
		if _, err := sshFn(machineName, script); err != nil {
			return fmt.Errorf("deploying file %s: %w", f.Path, err)
		}
	}
	// Re-evaluate sysctls
	_, _ = sshFn(machineName, "sudo sysctl --system")

	var enabledUnits []string
	for _, u := range conf.Systemd.Units {
		path := fmt.Sprintf("/etc/systemd/system/%s", u.Name)
		script := fmt.Sprintf("sudo tee %s >/dev/null <<'EOF'\n%s\nEOF\n", path, strings.TrimSpace(u.Contents))
		if _, err := sshFn(machineName, script); err != nil {
			return fmt.Errorf("deploying unit %s: %w", u.Name, err)
		}
		if u.Enabled {
			enabledUnits = append(enabledUnits, u.Name)
		}
	}

	if _, err := sshFn(machineName, "sudo systemctl daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}

	for _, u := range enabledUnits {
		if _, err := sshFn(machineName, "sudo systemctl enable --now "+u); err != nil {
			return fmt.Errorf("systemctl enable --now %s: %w", u, err)
		}
		_, _ = sshFn(machineName, "sudo systemctl restart "+u)
	}

	return nil
}
