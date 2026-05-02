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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var dbSeedRuntime string

var dbSeedCmd = &cobra.Command{
	Use:   "seed <engine>",
	Short: "Run the configured seed command against the local database",
	Long: `Executes the seed command configured in devx.yaml locally on the host, 
automatically injecting the database connection details.

 devx db seed postgres

Injected environment variables include the complete DATABASE_URL as well 
as the individual connection fragments required by legacy frameworks.`,
	Args: cobra.ExactArgs(1),
	RunE: runDbSeed,
}

func init() {
	dbSeedCmd.Flags().StringVar(&dbSeedRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	dbCmd.AddCommand(dbSeedCmd)
}

func runDbSeed(_ *cobra.Command, args []string) error {
	engineName := strings.ToLower(args[0])
	engine, ok := database.Registry[engineName]
	if !ok {
		return fmt.Errorf("unknown engine %q — supported: %s",
			engineName, strings.Join(database.SupportedEngines(), ", "))
	}

	// ── 1. Read devx.yaml ────────────────────────────────────────────────────
	// Idea 44: resolveConfig processes include blocks so databases from neighbouring
	// repos are visible and their Dir (working directory) is correctly set.
	cfg, err := resolveConfig("devx.yaml", "")
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("could not find devx.yaml in the current directory")
		}
		return fmt.Errorf("failed to read devx.yaml: %w", err)
	}

	seedCommand := ""
	var seedDir string // working directory for this seed (set if from an included project)
	for _, db := range cfg.Databases {
		if strings.ToLower(db.Engine) == engineName {
			seedCommand = db.Seed.Command
			seedDir = db.Dir
			break
		}
	}

	if seedCommand == "" {
		return fmt.Errorf("no 'seed.command' configured for %q in devx.yaml\n\n"+
			"Add a seed block under your database entry:\n\n"+
			"  databases:\n    - engine: %s\n      seed:\n        command: \"npm run db:seed\"",
			engineName, engineName)
	}

	containerName := fmt.Sprintf("devx-db-%s", engineName)
	runtime := dbSeedRuntime

	// ── 2. Verify container is running and resolve the real mapped host port ─

	// Format string explicitly grabs the first mapped HostPort for whichever
	// internal port the engine exposes. (e.g. 5432/tcp -> HostPort: 5433)
	// Example podman output: 5432/tcp -> 5432
	formatStr := fmt.Sprintf("{{range $p, $conf := .NetworkSettings.Ports}}{{if eq $p \"%d/tcp\"}}{{range $conf}}{{.HostPort}}{{end}}{{end}}{{end}}", engine.InternalPort)

	out, err := exec.Command(runtime, "inspect", "--format", formatStr, containerName).Output()
	if err != nil {
		// Try to fallback checking if it's running but port isn't exposed properly, or just report not running.
		checkCmd := exec.Command(runtime, "inspect", containerName, "--format", "{{.State.Running}}")
		if checkOut, checkErr := checkCmd.Output(); checkErr == nil && strings.TrimSpace(string(checkOut)) == "true" {
			return fmt.Errorf("the local %s database is running but its port (%d) does not appear to be exposed on the host", engineName, engine.InternalPort)
		}
		return fmt.Errorf("the local %s database is not running — start it with 'devx db spawn %s'",
			engineName, engineName)
	}

	portStr := strings.TrimSpace(string(out))
	if portStr == "" {
		return fmt.Errorf("could not determine the host port mapping for the %s container", engineName)
	}

	hostPort, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("failed to parse host port %q: %w", portStr, err)
	}

	connUrl := engine.ConnString(hostPort)

	// ── 3. Build inject environment ───────────────────────────────────────

	// Create completely isolated overlay map
	injectEnv := make(map[string]string)
	injectEnv["DATABASE_URL"] = connUrl
	injectEnv["DATABASE_HOST"] = "localhost"
	injectEnv["DATABASE_PORT"] = portStr

	for k, v := range engine.Env {
		injectEnv[k] = v
	}

	if outputJSON {
		type outFormat struct {
			Engine      string            `json:"engine"`
			Command     string            `json:"command"`
			InjectedEnv map[string]string `json:"injected_env"`
		}
		enc, _ := json.MarshalIndent(outFormat{
			Engine:      engineName,
			Command:     seedCommand,
			InjectedEnv: injectEnv,
		}, "", "  ")
		if DryRun {
			fmt.Println(string(enc))
			return nil
		}
	} else {
		if DryRun {
			fmt.Printf("[dry-run] Would execute: %s\n", seedCommand)
			fmt.Println("[dry-run] Injected Environment Variables:")
			for k, v := range injectEnv {
				// Hide passwords in output
				if strings.Contains(strings.ToLower(k), "pass") || strings.Contains(strings.ToLower(k), "secret") {
					fmt.Printf("          %s=********\n", k)
				} else {
					fmt.Printf("          %s=%s\n", k, v)
				}
			}
			return nil
		}

		// ── 4. Confirm ───────────────────────────────────────────────────
		if !NonInteractive {
			var confirmed bool
			envNotice := fmt.Sprintf("Injecting DATABASE_URL and %d engine variables", len(engine.Env)+2)
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title(fmt.Sprintf("Run seed command against %s?", engineName)).
						Description(fmt.Sprintf(
							"Command: %s\nPort:    %s\n\n%s",
							tui.StyleMuted.Render(seedCommand),
							tui.StyleMuted.Render(portStr),
							tui.StyleMuted.Render(envNotice),
						)).
						Affirmative("Yes, execute seed").
						Negative("Cancel").
						Value(&confirmed),
				),
			).WithTheme(huh.ThemeCatppuccin())

			if err := form.Run(); err != nil || !confirmed {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		fmt.Printf("\n%s Seeding %s database...\n", tui.IconRunning, engineName)
		fmt.Printf("  %s %s\n\n", tui.StyleMuted.Render("exec →"), seedCommand)
	}

	// ── 5. Setup execution ───────────────────────────────────────────────
	seedExec := exec.Command("sh", "-c", seedCommand)

	// Idea 44: run from the included project's directory if set
	if seedDir != "" {
		seedExec.Dir = seedDir
	}

	// Inherit host environment, but append ours
	hostEnv := os.Environ()
	for k, v := range injectEnv {
		hostEnv = append(hostEnv, fmt.Sprintf("%s=%s", k, v))
	}
	seedExec.Env = hostEnv

	var stderrBuf bytes.Buffer
	seedExec.Stdout = os.Stdout
	seedExec.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	// ── 6. Execute (with interactive gcloud auth recovery loop) ─────────
	for {
		err := seedExec.Run()
		if err == nil {
			break // Success
		}

		// If command failed, check if it's because of a gcloud auth timeout.
		if !NonInteractive {
			if recovered := devxerr.RecoverGcloudAuth(stderrBuf.String()); recovered {
				// User successfully re-authenticated, retry the command.
				fmt.Printf("\n%s Re-running seed command...\n\n", tui.IconRunning)

				// Recreate the command because exec.Cmd cannot be re-used after Run()
				seedExec = exec.Command("sh", "-c", seedCommand)
				seedExec.Env = hostEnv
				stderrBuf.Reset()
				seedExec.Stdout = os.Stdout
				seedExec.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
				continue
			}
		}

		return fmt.Errorf("seed command failed: %w", err)
	}

	if !outputJSON {
		fmt.Printf("\n%s %s database successfully seeded.\n", tui.IconDone, engineName)
	}

	return nil
}
