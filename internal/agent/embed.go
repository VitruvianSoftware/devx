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
	"antigravity": ".agent/skills",
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

	// Make sure the target directory exists (for things like .github/ or .agent/)
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
