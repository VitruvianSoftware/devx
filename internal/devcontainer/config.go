// Package devcontainer parses devcontainer.json files and resolves
// the container image, mounts, environment, and post-create commands
// needed to spin up an isolated development shell.
package devcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the subset of the devcontainer.json spec that
// devx supports. See https://containers.dev/implementors/json_reference/
type Config struct {
	Name              string                 `json:"name"`
	Image             string                 `json:"image"`
	WorkspaceFolder   string                 `json:"workspaceFolder"`
	ContainerEnv      map[string]string      `json:"containerEnv"`
	RemoteUser        string                 `json:"remoteUser"`
	PostCreateCommand interface{}            `json:"postCreateCommand"` // string or []string
	Mounts            []interface{}          `json:"mounts"`            // string or object
	ForwardPorts      []int                  `json:"forwardPorts"`
	Features          map[string]interface{} `json:"features"`
}

// PostCreateCmd returns the post-create command as a single string.
func (c *Config) PostCreateCmd() string {
	if c.PostCreateCommand == nil {
		return ""
	}
	switch v := c.PostCreateCommand.(type) {
	case string:
		return v
	case []interface{}:
		parts := make([]string, len(v))
		for i, p := range v {
			parts[i] = fmt.Sprintf("%v", p)
		}
		return joinArgs(parts)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " && "
		}
		result += a
	}
	return result
}

// Load reads and parses a devcontainer.json file.
// It searches the standard locations: .devcontainer/devcontainer.json,
// .devcontainer.json, and .devcontainer/<name>/devcontainer.json.
func Load(projectDir string) (*Config, string, error) {
	candidates := []string{
		filepath.Join(projectDir, ".devcontainer", "devcontainer.json"),
		filepath.Join(projectDir, ".devcontainer.json"),
	}

	// Also check for named configs under .devcontainer/*/devcontainer.json
	dcDir := filepath.Join(projectDir, ".devcontainer")
	if entries, err := os.ReadDir(dcDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				candidates = append(candidates, filepath.Join(dcDir, e.Name(), "devcontainer.json"))
			}
		}
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, path, fmt.Errorf("parsing %s: %w", path, err)
		}
		return &cfg, path, nil
	}

	return nil, "", fmt.Errorf("no devcontainer.json found in %s", projectDir)
}
