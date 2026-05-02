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

package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LocalConfig holds machine-local devx configuration that is NOT committed
// to version control. It lives at ~/.devx/config.yaml and provides overrides
// for project-level devx.yaml settings (e.g. which VM provider to use).
type LocalConfig struct {
	// Provider is the VM backend to use. Values: "auto", "podman", "lima",
	// "colima", "docker", "orbstack". Empty defaults to "auto".
	Provider string `yaml:"provider,omitempty"`
}

// LocalConfigDir returns the directory for machine-local devx configuration.
func LocalConfigDir() string {
	if d := os.Getenv("DEVX_CONFIG_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devx")
}

// LocalConfigPath returns the full path to the local config file.
func LocalConfigPath() string {
	return filepath.Join(LocalConfigDir(), "config.yaml")
}

// LoadLocal reads the machine-local config from ~/.devx/config.yaml.
// If the file doesn't exist, it returns a zero-value config (no error).
func LoadLocal() (*LocalConfig, error) {
	path := LocalConfigPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &LocalConfig{}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg LocalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveLocal writes the machine-local config to ~/.devx/config.yaml.
// It creates the directory if it doesn't exist.
func SaveLocal(cfg *LocalConfig) error {
	dir := LocalConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(LocalConfigPath(), data, 0644)
}
