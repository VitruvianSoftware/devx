package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var dbPullRuntime string
var dbPullJobs int

var dbPullCmd = &cobra.Command{
	Use:   "pull <engine>",
	Short: "Pull scrubbed production data and stream it into the local database",
	Long: `Executes the pull command configured in devx.yaml and streams the output
directly into the local database container — no temp files written to disk.

Two formats are supported, configured via 'format' in devx.yaml:

  sql (default) — plain SQL text, ingested via psql/mysql/mongorestore.
    Best for: standard pg_dump, mysqldump, mongodump.
    Import speed: sequential, single-threaded.

  custom — PostgreSQL binary custom format (pg_dump -Fc).
    Ingested via pg_restore with parallel -j workers.
    Best for: large databases (5GB+) where import time matters.
    Import speed: parallel, uses all CPU cores by default.

Example devx.yaml:

  databases:
    - engine: postgres
      pull:
        # Plain SQL (all engines):
        command: "gcloud storage cat gs://acme-dumps/latest.sql.gz | gunzip"

        # Or, binary parallel (postgres only):
        format: custom
        jobs: 4
        command: "gcloud storage cat gs://acme-dumps/latest.dump"
`,
	Args: cobra.ExactArgs(1),
	RunE: runDbPull,
}

func init() {
	dbPullCmd.Flags().StringVar(&dbPullRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	dbPullCmd.Flags().IntVarP(&dbPullJobs, "jobs", "j", 0,
		"Parallel import workers for custom format (0 = auto: number of CPUs)")
	dbCmd.AddCommand(dbPullCmd)
}

// dbPullYAML mirrors the pull section of devx.yaml databases entries.
type dbPullYAML struct {
	Databases []struct {
		Engine string `yaml:"engine"`
		Pull   struct {
			Command string `yaml:"command"`
			Format  string `yaml:"format"` // "sql" (default) or "custom"
			Jobs    int    `yaml:"jobs"`   // parallel workers for pg_restore (0 = nCPU)
		} `yaml:"pull"`
	} `yaml:"databases"`
}

func runDbPull(_ *cobra.Command, args []string) error {
	engineName := strings.ToLower(args[0])
	engine, ok := database.Registry[engineName]
	if !ok {
		return fmt.Errorf("unknown engine %q — supported: %s",
			engineName, strings.Join(database.SupportedEngines(), ", "))
	}

	// ── 1. Read devx.yaml ────────────────────────────────────────────────────
	yamlData, err := os.ReadFile("devx.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("could not find devx.yaml in the current directory")
		}
		return fmt.Errorf("failed to read devx.yaml: %w", err)
	}

	var cfg dbPullYAML
	if err := yaml.Unmarshal(yamlData, &cfg); err != nil {
		return fmt.Errorf("failed to parse devx.yaml: %w", err)
	}

	// Find the matching engine block
	pullCommand := ""
	pullFormat := "sql"
	pullJobs := 0
	for _, db := range cfg.Databases {
		if strings.ToLower(db.Engine) == engineName {
			pullCommand = db.Pull.Command
			if db.Pull.Format != "" {
				pullFormat = strings.ToLower(db.Pull.Format)
			}
			pullJobs = db.Pull.Jobs
			break
		}
	}

	if pullCommand == "" {
		return fmt.Errorf("no 'pull.command' configured for %q in devx.yaml\n\n"+
			"Add a pull block under your database entry:\n\n"+
			"  databases:\n    - engine: %s\n      pull:\n        command: \"your-dump-command-here\"",
			engineName, engineName)
	}

	// CLI flag overrides yaml for jobs
	if dbPullJobs > 0 {
		pullJobs = dbPullJobs
	}
	if pullJobs == 0 {
		pullJobs = runtime.NumCPU()
	}

	// Validate format
	if pullFormat != "sql" && pullFormat != "custom" {
		return fmt.Errorf("unknown format %q — use 'sql' or 'custom'", pullFormat)
	}
	if pullFormat == "custom" && engineName != "postgres" {
		return fmt.Errorf("format 'custom' (pg_restore) is only supported for postgres")
	}

	containerName := fmt.Sprintf("devx-db-%s", engineName)

	// ── 2. Verify container is running ────────────────────────────────────────
	out, err := exec.Command(dbPullRuntime, "inspect", containerName, "--format", "{{.State.Running}}").Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("the local %s database is not running — start it with 'devx db spawn %s'",
			engineName, engineName)
	}

	if DryRun {
		formatDesc := "SQL text mode (psql)"
		if pullFormat == "custom" {
			formatDesc = fmt.Sprintf("binary format (pg_restore -j %d)", pullJobs)
		}
		fmt.Printf("[dry-run] Would execute: %s\n", pullCommand)
		fmt.Printf("[dry-run] Ingestion:     %s → %s  [%s]\n", pullCommand, containerName, formatDesc)
		return nil
	}

	// ── 3. Confirm ───────────────────────────────────────────────────────────
	if !NonInteractive {
		formatLabel := "SQL text (sequential)"
		if pullFormat == "custom" {
			formatLabel = fmt.Sprintf("binary/custom (pg_restore -j %d parallel workers)", pullJobs)
		}

		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Pull and restore %s database?", engineName)).
					Description(fmt.Sprintf(
						"Pull command:  %s\nFormat:        %s\n\nThis will stream the dump directly into your local container.",
						tui.StyleMuted.Render(pullCommand),
						tui.StyleMuted.Render(formatLabel),
					)).
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

	fmt.Printf("\n%s Pulling %s data", tui.IconRunning, engineName)
	if pullFormat == "custom" {
		fmt.Printf(" (binary mode, %d parallel workers)", pullJobs)
	}
	fmt.Println("...")

	// ── 4. Build the ingestion command ────────────────────────────────────────
	var ingestArgs []string
	switch {
	case engineName == "postgres" && pullFormat == "custom":
		// Binary custom format: pg_restore with parallel jobs
		// pg_restore reads the dump as binary from stdin (--format=custom)
		// It can parallelise reads (-j N) even from a stream since Postgres 14+
		ingestArgs = []string{
			"exec", "-i", containerName,
			"pg_restore",
			"--no-owner",          // don't try to assign DB ownership
			"--no-privileges",     // skip GRANT/REVOKE — local dev doesn't need ACLs
			"-d", engine.Env["POSTGRES_DB"],
			"-U", engine.Env["POSTGRES_USER"],
			fmt.Sprintf("-j%d", pullJobs),
			"-v",
			"--format=custom",
			"--single-transaction",
		}
	case engineName == "postgres":
		ingestArgs = []string{"exec", "-i", containerName,
			"psql", "-U", engine.Env["POSTGRES_USER"], "-d", engine.Env["POSTGRES_DB"]}
	case engineName == "mysql":
		ingestArgs = []string{"exec", "-i", containerName,
			"mysql", "-u", engine.Env["MYSQL_USER"], "-p" + engine.Env["MYSQL_ROOT_PASSWORD"],
			engine.Env["MYSQL_DATABASE"]}
	case engineName == "redis":
		ingestArgs = []string{"exec", "-i", containerName, "redis-cli"}
	case engineName == "mongo":
		ingestArgs = []string{"exec", "-i", containerName,
			"mongorestore",
			"--username", engine.Env["MONGO_INITDB_ROOT_USERNAME"],
			"--password", engine.Env["MONGO_INITDB_ROOT_PASSWORD"],
			"--authenticationDatabase", "admin",
			"--archive"}
	default:
		return fmt.Errorf("pull ingestion not yet supported for engine %q", engineName)
	}

	// ── 5. Wire the pipeline: pull stdout → container stdin ──────────────────
	pullProc := exec.Command("sh", "-c", pullCommand)
	ingestProc := exec.Command(dbPullRuntime, ingestArgs...)

	pr, pw := io.Pipe()
	pullProc.Stdout = pw
	ingestProc.Stdin = pr

	// Surface both stderr streams so devs can see auth errors / psql notices
	pullProc.Stderr = os.Stderr
	ingestProc.Stderr = os.Stderr
	ingestProc.Stdout = os.Stdout

	fmt.Printf("\n  %s %s\n", tui.StyleMuted.Render("pull →"), pullCommand)
	fmt.Printf("  %s %s %s\n\n",
		tui.StyleMuted.Render("ingest →"),
		dbPullRuntime,
		strings.Join(ingestArgs, " "),
	)

	if err := ingestProc.Start(); err != nil {
		return fmt.Errorf("failed to start ingestion process: %w", err)
	}
	if err := pullProc.Start(); err != nil {
		return fmt.Errorf("failed to start pull command: %w", err)
	}

	pullErr := pullProc.Wait()
	_ = pw.Close()
	ingestErr := ingestProc.Wait()

	if pullErr != nil {
		return fmt.Errorf("pull command failed: %w", pullErr)
	}
	if ingestErr != nil {
		return fmt.Errorf("ingestion into container failed: %w", ingestErr)
	}

	fmt.Printf("\n%s %s database successfully restored.\n", tui.IconDone, engineName)
	return nil
}
