package agent

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed all:templates
var EmbedFS embed.FS

var ManifestPaths = map[string]string{
	"cursor":      ".cursor/skills/devx/SKILL.md",
	"claude":      ".claude/skills/devx/SKILL.md",
	"copilot":     ".github/skills/devx/SKILL.md",
	"antigravity": ".agent/skills/devx/SKILL.md", // Standard format based on UI
}

// Install copies the correct template string to the local working directory.
func Install(targetAgent string) error {
	relPath, ok := ManifestPaths[targetAgent]
	if !ok {
		return fmt.Errorf("unsupported agent: %s", targetAgent)
	}

	content, err := EmbedFS.ReadFile("templates/" + relPath)
	if err != nil {
		return fmt.Errorf("reading embedded template %q: %w", relPath, err)
	}

	// Make sure the target directory exists (for things like .github/ or .agent/)
	if err := os.MkdirAll(filepath.Dir(relPath), 0755); err != nil {
		return fmt.Errorf("creating directory for %q: %w", relPath, err)
	}

	// Check if file exists to prevent hard-overwriting without confirmation
	if _, err := os.Stat(relPath); err == nil {
		fmt.Printf("⚠️  File %s already exists. Skipping.\n", relPath)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %q: %w", relPath, err)
	}

	if err := os.WriteFile(relPath, content, 0644); err != nil {
		return fmt.Errorf("writing %q: %w", relPath, err)
	}

	fmt.Printf("✓ Installed %s AI Agent config: %s\n", targetAgent, relPath)
	return nil
}
