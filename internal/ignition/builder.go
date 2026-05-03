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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Build substitutes variables into the Butane template, compiles it via
// butane, writes the Ignition JSON to a temp file, and returns its path.
// The caller is responsible for removing the temp file when done.
func Build(templatePath, tunnelToken, tunnelID, hostname, cfDomain, runtime string) (string, error) {
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("reading Butane template %s: %w", templatePath, err)
	}

	// Find SSH public key for Podman access
	sshPubKey := generateOrFindSSHKey()

	// Substitute ${VAR} placeholders using os.Expand
	vars := map[string]string{
		"CF_TUNNEL_TOKEN": tunnelToken,
		"CF_TUNNEL_ID":    tunnelID,
		"DEV_HOSTNAME":    hostname,
		"CF_DOMAIN":       cfDomain,
		"SSH_PUB_KEY":     sshPubKey,
		"RUNTIME":         runtime,
	}
	populated := os.Expand(string(templateData), func(key string) string {
		if v, ok := vars[key]; ok {
			return v
		}
		return "${" + key + "}" // leave unknown vars intact
	})

	// Write populated Butane to a temp file
	buFile, err := os.CreateTemp("", "devx-*.bu")
	if err != nil {
		return "", fmt.Errorf("creating temp Butane file: %w", err)
	}
	defer func() { _ = os.Remove(buFile.Name()) }()  // clean up input after compilation

	if _, err := buFile.WriteString(populated); err != nil {
		_ = buFile.Close()
		return "", fmt.Errorf("writing Butane file: %w", err)
	}
	_ = buFile.Close()

	// Write Ignition output to a temp file
	ignFile, err := os.CreateTemp("", "devx-*.ign")
	if err != nil {
		return "", fmt.Errorf("creating temp Ignition file: %w", err)
	}
	ignPath := ignFile.Name()
	_ = ignFile.Close()

	// Compile with butane
	var stderr bytes.Buffer
	cmd := exec.Command("butane", "--pretty", "--strict", "--output", ignPath, buFile.Name())
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(ignPath)
		return "", fmt.Errorf("butane compile failed: %w\n%s", err, stderr.String())
	}

	return ignPath, nil
}

// Validate checks that the template compiles cleanly with dummy values.
// Useful for CI.
func Validate(templatePath string) error {
	path, err := Build(templatePath, "dummy-token", "dummy-id", "test-machine", "test.domain", "podman")
	if err != nil {
		return err
	}
	_ = os.Remove(path)
	return nil
}

func generateOrFindSSHKey() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Try podman's internal machine key
	paths := []string{
		filepath.Join(home, ".local/share/containers/podman/machine/machine.pub"),
		filepath.Join(home, ".ssh/podman-machine-default.pub"),
		filepath.Join(home, ".ssh/id_ed25519.pub"),
		filepath.Join(home, ".ssh/id_rsa.pub"),
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}
