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

package agent

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed all:templates
var EmbedFS embed.FS

var AgentBasePaths = map[string]string{
	"cursor":      ".cursor/skills",
	"claude":      ".claude/skills",
	"copilot":     ".github/skills",
	"antigravity": ".agents/skills",
}

var AvailableSkills = []struct {
	ID          string
	Name        string
	Description string
}{
	{
		ID:          "devx",
		Name:        "Devx CLI Orchestrator Rules",
		Description: "Mandates --json, --dry-run, and handles prediction of devx exit codes.",
	},
	{
		ID:          "platform-engineer",
		Name:        "Platform Engineering SOP (Mandatory Docs)",
		Description: "Enforces strict documentation-first behavior and image embedding requirements for AI agents.",
	},
}

// Install copies the correct template string to the local working directory.
func Install(targetAgent string, skillName string, force bool) error {
	basePath, ok := AgentBasePaths[targetAgent]
	if !ok {
		return fmt.Errorf("unsupported agent: %s", targetAgent)
	}

	relPath := fmt.Sprintf("%s/%s/SKILL.md", basePath, skillName)
	templatePath := "templates/" + relPath

	content, err := EmbedFS.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading embedded template %q: %w", templatePath, err)
	}

	// Make sure the target directory exists (for things like .github/ or .agents/)
	if err := os.MkdirAll(filepath.Dir(relPath), 0755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", relPath, err)
	}

	// Check if file exists to prevent hard-overwriting without confirmation
	if !force {
		if _, err := os.Stat(relPath); err == nil {
			fmt.Printf("⚠️  File %s already exists. Skipping. (use --force to overwrite)\n", relPath)
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %q: %w", relPath, err)
		}
	}

	if err := os.WriteFile(relPath, content, 0644); err != nil {
		return fmt.Errorf("writing %q: %w", relPath, err)
	}

	fmt.Printf("✓ Installed %s AI Agent skill: %s\n", targetAgent, relPath)
	return nil
}
