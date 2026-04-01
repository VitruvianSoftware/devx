package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var dbPullRuntime string

var dbPullCmd = &cobra.Command{
	Use:   "pull <engine>",
	Short: "Pull scrubbed production data and stream it into the local database",
	Long: `Executes the pull command defined in your devx.yaml and streams the
resulting anonymized data dump directly into your local database container.

This avoids writing massive raw SQL files to your disk and ensures you
always test against realistic (but safe) staging/production data shapes.

Example devx.yaml configuration:
databases:
  - engine: postgres
    port: 5432
    pull:
      command: "gcloud storage cat gs://acme-scrubbed-dumps/latest.sql.gz | gunzip"
`,
	Args: cobra.ExactArgs(1),
	RunE: runDbPull,
}

func init() {
	dbPullCmd.Flags().StringVar(&dbPullRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	dbCmd.AddCommand(dbPullCmd)
}

func runDbPull(_ *cobra.Command, args []string) error {
	engineName := strings.ToLower(args[0])
	engine, ok := database.Registry[engineName]
	if !ok {
		return fmt.Errorf("unknown engine %q — supported: %s",
			engineName, strings.Join(database.SupportedEngines(), ", "))
	}

	// 1. Read devx.yaml
	yamlData, err := os.ReadFile("devx.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("could not find devx.yaml in the current directory")
		}
		return fmt.Errorf("failed to read devx.yaml: %w", err)
	}

	// Parse out just the databases > pull block
	type dbPullConfig struct {
		Databases []struct {
			Engine string `yaml:"engine"`
			Pull   struct {
				Command string `yaml:"command"`
			} `yaml:"pull"`
		} `yaml:"databases"`
	}

	var cfg dbPullConfig
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		return fmt.Errorf("failed to parse databases from devx.yaml: %w", err)
	}

	// Find the matching engine in the yaml
	pullCommand := ""
	for _, db := range cfg.Databases {
		if strings.ToLower(db.Engine) == engineName {
			pullCommand = db.Pull.Command
			break
		}
	}

	if pullCommand == "" {
		return fmt.Errorf("no 'pull.command' configured for %q in devx.yaml", engineName)
	}

	containerName := fmt.Sprintf("devx-db-%s", engineName)

	// 2. Ensure container is running
	checkCmd := exec.Command(dbPullRuntime, "inspect", containerName, "--format", "{{.State.Running}}")
	if out, err := checkCmd.Output(); err != nil || strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("the local %s database is not running. Start it first with 'devx db spawn %s'", engineName, engineName)
	}

	if DryRun {
		fmt.Printf("DRY RUN: Would execute `%s` and pipe output into %s\n", pullCommand, containerName)
		return nil
	}

	// 3. Confirm with user
	if !NonInteractive {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Pull and overwrite %s database?", engineName)).
					Description(fmt.Sprintf("This will download data via the configured pull command:\n\n%s\n\nEnsure your dump file drops/truncates cleanly to avoid table merging issues.", tui.StyleMuted.Render(pullCommand))).
					Affirmative("Yes, pull data").
					Negative("Cancel").
					Value(&confirmed),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Printf("\n%s Pulling %s data via script...\n", tui.IconRunning, engineName)

	// 4. Set up the pipeline: script stdout -> container stdin
	pullProc := exec.Command("sh", "-c", pullCommand)
	
	// Determine the ingest command for the container
	var ingestArgs []string
	switch engineName {
	case "postgres":
		ingestArgs = []string{"exec", "-i", containerName, "psql", "-U", engine.Env["POSTGRES_USER"], "-d", engine.Env["POSTGRES_DB"]}
	case "mysql":
		ingestArgs = []string{"exec", "-i", containerName, "mysql", "-u", engine.Env["MYSQL_USER"], "-p" + engine.Env["MYSQL_ROOT_PASSWORD"], engine.Env["MYSQL_DATABASE"]}
	case "redis":
		ingestArgs = []string{"exec", "-i", containerName, "redis-cli"}
	case "mongo":
		ingestArgs = []string{"exec", "-i", containerName, "mongorestore", "--username", engine.Env["MONGO_INITDB_ROOT_USERNAME"], "--password", engine.Env["MONGO_INITDB_ROOT_PASSWORD"], "--authenticationDatabase", "admin", "--archive"}
	default:
		return fmt.Errorf("pull ingestion is not supported natively for engine %q", engineName)
	}

	ingestProc := exec.Command(dbPullRuntime, ingestArgs...)

	// Pipe the output
	pr, pw := io.Pipe()
	pullProc.Stdout = pw
	ingestProc.Stdin = pr
	
	// Wire errors to user screen so they can debug GCP/AWS auth issues or psql syntax issues
	pullProc.Stderr = os.Stderr
	ingestProc.Stderr = os.Stderr
	ingestProc.Stdout = os.Stdout

	fmt.Printf("  %s %s\n", tui.StyleMuted.Render("→"), pullCommand)
	fmt.Printf("  %s %s %s\n\n", tui.StyleMuted.Render("→"), dbPullRuntime, strings.Join(ingestArgs, " "))

	// Start ingestion first
	if err := ingestProc.Start(); err != nil {
		return fmt.Errorf("failed to start ingestion process into container: %w", err)
	}

	// Start pulling
	if err := pullProc.Start(); err != nil {
		return fmt.Errorf("failed to start pull command: %w", err)
	}

	// Wait for pull to finish and close the pipe
	pullErr := pullProc.Wait()
	_ = pw.Close()

	// Wait for import to wrap up
	ingestErr := ingestProc.Wait()

	if pullErr != nil {
		return fmt.Errorf("pull script failed: %v", pullErr)
	}
	if ingestErr != nil {
		return fmt.Errorf("data ingestion into container failed: %v", ingestErr)
	}

	fmt.Printf("\n%s Database %s successfully restored and imported.\n", tui.IconDone, engineName)
	return nil
}
