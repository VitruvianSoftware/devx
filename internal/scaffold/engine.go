package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

//go:embed all:templates
var EmbedFS embed.FS

// Vars holds all substitution variables available inside every .tmpl file.
type Vars struct {
	ProjectName string // e.g. "my-service"
	ProjectSlug string // snake_case version, e.g. "my_service"
	ModulePath  string // Go module path, e.g. "github.com/acme/go-my-service"
	Author      string // e.g. "James Nguyen"
	Year        string // e.g. "2026"
	Domain      string // e.g. "ipv1337.dev"
	GoVersion   string // e.g. "1.22"
	Description string // one-line project description
	HasDatabase bool   // whether the template provisions a database
	DBEngine    string // e.g. "postgres"
}

// Template describes a single embedded scaffold template.
type Template struct {
	ID          string // matches the directory name under templates/
	Name        string // human-readable display name
	Description string // shown in the interactive picker
	Language    string // "go", "node", "python", ""
	HasDatabase bool   // whether this template provisions a DB
	DBEngine    string // default DB engine for this template
}

// Registry contains all built-in templates.
var Registry = []Template{
	{
		ID:          "go-api",
		Name:        "Go REST API",
		Description: "Production-ready Go HTTP API — Chi router, PostgreSQL, migrations, Docker, CI",
		Language:    "go",
		HasDatabase: true,
		DBEngine:    "postgres",
	},
	{
		ID:          "go-cli",
		Name:        "Go CLI Tool",
		Description: "Go CLI tool — Cobra, GoReleaser, cross-platform CI pipeline",
		Language:    "go",
		HasDatabase: false,
	},
	{
		ID:          "node-api",
		Name:        "Node.js REST API",
		Description: "TypeScript Express API — Prisma ORM, PostgreSQL, Jest, Docker, CI",
		Language:    "node",
		HasDatabase: true,
		DBEngine:    "postgres",
	},
	{
		ID:          "next-app",
		Name:        "Next.js Full-Stack App",
		Description: "Next.js 14 App Router — TypeScript, Prisma, PostgreSQL, Tailwind, CI",
		Language:    "node",
		HasDatabase: true,
		DBEngine:    "postgres",
	},
	{
		ID:          "python-api",
		Name:        "Python FastAPI",
		Description: "FastAPI — SQLAlchemy, Alembic migrations, PostgreSQL, Docker, CI",
		Language:    "python",
		HasDatabase: true,
		DBEngine:    "postgres",
	},
	{
		ID:          "empty",
		Name:        "Empty (devx-only)",
		Description: "Bare-minimum devx.yaml + .gitignore + README — bring your own stack",
		Language:    "",
		HasDatabase: false,
	},
}

// Find returns the Template with the given ID.
func Find(id string) (Template, bool) {
	for _, t := range Registry {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}

// Scaffold renders the template with id into targetDir using the provided vars.
// Files ending in .tmpl are rendered; all other files are copied verbatim.
func Scaffold(templateID, targetDir string, vars Vars) error {
	// Default year if not set
	if vars.Year == "" {
		vars.Year = time.Now().Format("2006")
	}

	// Derive slug (snake_case) from project name
	if vars.ProjectSlug == "" {
		vars.ProjectSlug = toSlug(vars.ProjectName)
	}

	root := filepath.Join("templates", templateID)

	return fs.WalkDir(EmbedFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute relative path from the template root
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Skip the template root itself
		if rel == "." {
			return nil
		}

		targetPath := filepath.Join(targetDir, rel)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		// Strip .tmpl extension from the output path
		outputPath := targetPath
		isTemplate := strings.HasSuffix(targetPath, ".tmpl")
		if isTemplate {
			outputPath = strings.TrimSuffix(targetPath, ".tmpl")
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(outputPath), err)
		}

		// Read embedded content
		content, err := EmbedFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		if isTemplate {
			// Parse and execute through text/template
			tmpl, err := template.New(filepath.Base(path)).Parse(string(content))
			if err != nil {
				return fmt.Errorf("parse template %s: %w", path, err)
			}

			f, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("create %s: %w", outputPath, err)
			}
			defer f.Close()

			if err := tmpl.Execute(f, vars); err != nil {
				return fmt.Errorf("render %s: %w", path, err)
			}
		} else {
			// Copy verbatim
			if err := os.WriteFile(outputPath, content, 0644); err != nil {
				return fmt.Errorf("write %s: %w", outputPath, err)
			}
		}

		return nil
	})
}

// PostScaffold runs git init and language-specific setup in targetDir.
func PostScaffold(templateID, targetDir string) error {
	tmpl, ok := Find(templateID)
	if !ok {
		return nil
	}

	// Always git init
	if err := runIn(targetDir, "git", "init", "-q"); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	if err := runIn(targetDir, "git", "add", "."); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Language-specific hooks
	switch tmpl.Language {
	case "go":
		// go mod tidy to resolve dependencies
		_ = runIn(targetDir, "go", "mod", "tidy")
	case "node":
		// Only install if npm is available — silently skip otherwise
		if _, err := exec.LookPath("npm"); err == nil {
			_ = runIn(targetDir, "npm", "install", "--silent")
		}
	case "python":
		// Create virtualenv if python3 available
		if _, err := exec.LookPath("python3"); err == nil {
			_ = runIn(targetDir, "python3", "-m", "venv", ".venv")
		}
	}

	return nil
}

// GitAuthor returns the user's configured git name, falling back to the OS user.
func GitAuthor() string {
	out, err := exec.Command("git", "config", "--global", "user.name").Output()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		return strings.TrimSpace(string(out))
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "Developer"
}

// runIn executes a command inside dir, suppressing output.
func runIn(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// toSlug converts "my-project-name" → "my_project_name".
func toSlug(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}
