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

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/scaffold"
)

var scaffoldDir string
var scaffoldAuthor string
var scaffoldDomain string
var scaffoldNoGit bool
var scaffoldModulePath string
var scaffoldDescription string
var scaffoldForce bool

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold [template] [project-name]",
	GroupID: "orchestration",
	Short: "Generate a new project from a pre-wired template",
	Long: `Scaffold a new repository from a built-in devx template.

Templates include a devx.yaml topology, Dockerfile, CI pipeline,
database migrations, and development seed data pre-configured and
ready to run with 'devx up'.

Examples:
  # Interactive mode
  devx scaffold

  # Non-interactive (for AI agents)
  devx scaffold go-api my-service --module github.com/acme/go-my-service -y

  # List available templates
  devx scaffold list`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runScaffold,
}

var scaffoldListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available scaffold templates",
	RunE:  runScaffoldList,
}

func init() {
	scaffoldCmd.Flags().StringVar(&scaffoldDir, "dir", "", "Output directory (default: ./<project-name>)")
	scaffoldCmd.Flags().StringVar(&scaffoldAuthor, "author", "", "Author name (default: git config user.name)")
	scaffoldCmd.Flags().StringVar(&scaffoldDomain, "domain", "", "Cloudflare domain for devx.yaml (default: ipv1337.dev)")
	scaffoldCmd.Flags().StringVar(&scaffoldModulePath, "module", "", "Go module path (e.g. github.com/acme/go-my-service)")
	scaffoldCmd.Flags().StringVar(&scaffoldDescription, "desc", "", "One-line project description")
	scaffoldCmd.Flags().BoolVar(&scaffoldNoGit, "no-git", false, "Skip 'git init' after scaffolding")
	scaffoldCmd.Flags().BoolVarP(&scaffoldForce, "force", "f", false, "Overwrite existing files (default: skip files that already exist)")

	scaffoldCmd.AddCommand(scaffoldListCmd)
	rootCmd.AddCommand(scaffoldCmd)
}

func runScaffold(cmd *cobra.Command, args []string) error {
	var templateID, projectName string

	// Parse positional args
	if len(args) >= 1 {
		templateID = args[0]
	}
	if len(args) >= 2 {
		projectName = args[1]
	}

	// Determine author
	author := scaffoldAuthor
	if author == "" {
		author = scaffold.GitAuthor()
	}

	// Determine domain
	domain := scaffoldDomain
	if domain == "" {
		domain = "ipv1337.dev"
	}

	// Go version from runtime
	goVer := strings.TrimPrefix(runtime.Version(), "go")
	// Trim patch version (e.g. "1.22.3" → "1.22")
	parts := strings.Split(goVer, ".")
	if len(parts) >= 2 {
		goVer = parts[0] + "." + parts[1]
	}

	// Interactive mode when args are missing and we're in a TTY
	if !NonInteractive && (templateID == "" || projectName == "") {
		if err := runScaffoldInteractive(&templateID, &projectName, &scaffoldDescription, &scaffoldModulePath); err != nil {
			return err
		}
	}

	// Validate
	if templateID == "" {
		return fmt.Errorf("template is required — run 'devx scaffold list' to see options")
	}
	if projectName == "" {
		return fmt.Errorf("project-name is required")
	}

	tmpl, ok := scaffold.Find(templateID)
	if !ok {
		return fmt.Errorf("unknown template %q — run 'devx scaffold list'", templateID)
	}

	// Determine module path for Go projects
	modulePath := scaffoldModulePath
	if tmpl.Language == "go" && modulePath == "" {
		// Default suggestion
		modulePath = fmt.Sprintf("github.com/VitruvianSoftware/go-%s", projectName)
	}

	// Determine output directory
	outDir := scaffoldDir
	if outDir == "" {
		outDir = filepath.Join(".", projectName)
	}

	// Remove the directory-level overwrite guard — idempotency is now
	// handled per-file inside the engine. --force opts into overwriting.

	description := scaffoldDescription
	if description == "" {
		description = fmt.Sprintf("A %s project", tmpl.Name)
	}

	vars := scaffold.Vars{
		ProjectName: projectName,
		ModulePath:  modulePath,
		Author:      author,
		Domain:      domain,
		GoVersion:   goVer,
		Description: description,
		HasDatabase: tmpl.HasDatabase,
		DBEngine:    tmpl.DBEngine,
	}

	fmt.Printf("\n🏗️  Scaffolding %s → %s\n\n", tmpl.Name, outDir)

	result, err := scaffold.Scaffold(templateID, outDir, vars, scaffoldForce)
	if err != nil {
		return fmt.Errorf("scaffold failed: %w", err)
	}

	if !scaffoldNoGit {
		if err := scaffold.PostScaffold(templateID, outDir); err != nil {
			// Non-fatal: git init failure shouldn't block the developer
			_, _ = fmt.Fprintf(os.Stderr, "⚠️  Post-scaffold hooks: %v\n", err)
		}
	}

	printScaffoldSuccess(tmpl, projectName, outDir, vars, result)

	// Proactively offer devx doctor — default YES
	if !NonInteractive {
		var runDoctor bool
		err := huh.NewConfirm().
			Title("Run 'devx doctor' to verify your environment is ready?").
			Description("Checks that all tools for your devx features are installed.").
			Affirmative("Yes, check now").
			Negative("No, skip").
			Value(&runDoctor).
			WithTheme(huh.ThemeCatppuccin()).
			Run()
		if err == nil && runDoctor {
			doctorCmd := exec.Command(os.Args[0], "doctor")
			doctorCmd.Stdout = os.Stdout
			doctorCmd.Stderr = os.Stderr
			_ = doctorCmd.Run()
		}
	}

	return nil
}

func runScaffoldInteractive(templateID, projectName, description, modulePath *string) error {
	// Build template options for the picker
	opts := make([]huh.Option[string], len(scaffold.Registry))
	for i, t := range scaffold.Registry {
		opts[i] = huh.NewOption(fmt.Sprintf("%-12s  %s", t.Name, t.Description), t.ID)
	}

	nameVal := *projectName
	descVal := *description

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a template").
				Options(opts...).
				Value(templateID),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Description("Lowercase, hyphens allowed  (e.g. my-service)").
				Placeholder("my-project").
				Value(&nameVal),
			huh.NewInput().
				Title("Description").
				Description("One-line summary of what this project does").
				Placeholder("A high-performance REST API").
				Value(&descVal),
		),
	).WithTheme(huh.ThemeCatppuccin())

	if err := form.Run(); err != nil {
		return fmt.Errorf("scaffold cancelled")
	}

	*projectName = nameVal
	*description = descVal

	// For Go templates, prompt for module path with smart default
	if tmpl, ok := scaffold.Find(*templateID); ok && tmpl.Language == "go" && *modulePath == "" {
		suggested := fmt.Sprintf("github.com/VitruvianSoftware/go-%s", nameVal)
		modVal := suggested
		modForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Go module path").
					Description("Used in go.mod and import paths — customize your GitHub org if needed").
					Value(&modVal),
			),
		).WithTheme(huh.ThemeCatppuccin())
		if err := modForm.Run(); err == nil {
			*modulePath = modVal
		}
	}

	return nil
}

func printScaffoldSuccess(tmpl scaffold.Template, project, outDir string, vars scaffold.Vars, result scaffold.Result) {
	if outputJSON {
		type jsonResult struct {
			Template   string   `json:"template"`
			Project    string   `json:"project"`
			Directory  string   `json:"directory"`
			ModulePath string   `json:"module_path,omitempty"`
			Written    []string `json:"written"`
			Skipped    []string `json:"skipped"`
		}
		b, _ := json.MarshalIndent(jsonResult{
			Template:   tmpl.ID,
			Project:    project,
			Directory:  outDir,
			ModulePath: vars.ModulePath,
			Written:    result.Written,
			Skipped:    result.Skipped,
		}, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("✅ Project '%s' scaffolded at %s\n", project, outDir)
	fmt.Printf("   %d file(s) written", len(result.Written))
	if len(result.Skipped) > 0 {
		fmt.Printf(", %d skipped (already exist — use --force to overwrite)", len(result.Skipped))
	}
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Printf("    cd %s\n", outDir)
	fmt.Println("    devx up                  # Start database + tunnel")
	if tmpl.HasDatabase {
		fmt.Println("    psql $DATABASE_URL -f db/migrations/001_initial.sql")
		fmt.Println("    psql $DATABASE_URL -f db/seeds/001_sample_data.sql")
	}
	switch tmpl.Language {
	case "go":
		fmt.Println("    go run . serve           # Start the server")
	case "node":
		fmt.Println("    npm run dev              # Start with hot-reload")
	case "python":
		fmt.Println("    uvicorn app.main:app --reload")
	}
	fmt.Printf("\n  📖 Docs: https://devx.ipv1337.dev/guide/scaffold\n\n")
}

func runScaffoldList(_ *cobra.Command, _ []string) error {
	if outputJSON {
		type entry struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Language    string `json:"language"`
			HasDatabase bool   `json:"has_database"`
		}
		entries := make([]entry, len(scaffold.Registry))
		for i, t := range scaffold.Registry {
			entries[i] = entry{
				ID:          t.ID,
				Name:        t.Name,
				Description: t.Description,
				Language:    t.Language,
				HasDatabase: t.HasDatabase,
			}
		}
		b, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	fmt.Printf("\n  Available scaffold templates:\n\n")
	for _, t := range scaffold.Registry {
		dbTag := ""
		if t.HasDatabase {
			dbTag = fmt.Sprintf("  [%s]", t.DBEngine)
		}
		fmt.Printf("  %-12s  %s%s\n", t.ID, t.Description, dbTag)
	}
	fmt.Println("\n  Usage: devx scaffold <template> <project-name>")
	fmt.Printf("         devx scaffold --help\n\n")
	return nil
}
