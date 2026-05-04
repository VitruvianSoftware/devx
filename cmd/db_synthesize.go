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
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/ai"
	"github.com/VitruvianSoftware/devx/internal/database"
	"github.com/VitruvianSoftware/devx/internal/devxerr"
	"github.com/VitruvianSoftware/devx/internal/tui"
)

var synthRecords int
var synthModel string
var synthRuntime string

var dbSynthesizeCmd = &cobra.Command{
	Use:   "synthesize <engine>",
	Short: "Generate realistic synthetic data using AI",
	Long: `Generate highly realistic, edge-case-heavy synthetic data by extracting
your local database schema and sending it to an AI model. The generated
SQL INSERT statements are piped directly into your local container.

Supported engines: postgres, mysql

AI Provider Priority:
  1. Local Ollama (port 11434)
  2. Local LM Studio (port 1234)
  3. OPENAI_API_KEY environment variable (cloud fallback)

The generated data intentionally includes edge cases like Unicode characters,
very long strings, NULL values, and extreme numeric ranges to help catch bugs
that "perfect" seed data misses.`,
	Example: `  # Generate 100 synthetic records for PostgreSQL
  devx db synthesize postgres

  # Generate 50 records using a specific model
  devx db synthesize mysql --records 50 --model llama3

  # Preview the schema and prompt without generating data
  devx db synthesize postgres --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runDBSynthesize,
}

func init() {
	dbSynthesizeCmd.Flags().IntVar(&synthRecords, "records", 100,
		"Number of synthetic records to generate")
	dbSynthesizeCmd.Flags().StringVar(&synthModel, "model", "",
		"Target LLM model (default: provider's default)")
	dbSynthesizeCmd.Flags().StringVar(&synthRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	dbCmd.AddCommand(dbSynthesizeCmd)
}

// synthesizeSystemPrompt is the fixed system prompt for SQL generation.
const synthesizeSystemPrompt = `You are an expert database administrator. Generate realistic, chaotic synthetic data as raw SQL INSERT statements. Rules:
1. Output ONLY valid SQL. No markdown, no explanations, no code blocks, no comments.
2. Include edge cases: Unicode characters (Japanese, Arabic, emoji), very long strings (200+ chars), NULL values where columns are nullable, minimum and maximum numeric values, dates spanning decades.
3. Respect foreign key relationships: insert parent rows before child rows.
4. Wrap all statements in BEGIN; ... COMMIT;
5. Use the exact column names from the schema provided.`

func runDBSynthesize(_ *cobra.Command, args []string) error {
	engineName := strings.ToLower(args[0])

	// ── 1. Validate engine ──────────────────────────────────────────────────
	if !database.IsSynthesizable(engineName) {
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBSynthUnsupported,
			Message: fmt.Sprintf("engine %q does not support synthetic data generation — supported: %s",
				engineName, strings.Join(database.SynthesizableEngines(), ", ")),
		}
	}

	runtime := synthRuntime
	containerName := fmt.Sprintf("devx-db-%s", engineName)

	// ── 2. Verify container is running ──────────────────────────────────────
	checkCmd := exec.Command(runtime, "inspect", containerName, "--format", "{{.State.Running}}")
	if out, err := checkCmd.Output(); err != nil || strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("the local %s database is not running — start it with 'devx db spawn %s'",
			engineName, engineName)
	}

	// ── 3. Discover AI provider ─────────────────────────────────────────────
	provider := ai.DiscoverAIProvider()
	if provider == nil {
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBSynthNoAI,
			Message:  "No AI provider found. Start Ollama (ollama serve) or export OPENAI_API_KEY.",
		}
	}

	// ── 4. Extract schema ───────────────────────────────────────────────────
	if !outputJSON {
		fmt.Printf("📋 Extracting schema from %s...\n", engineName)
	}

	schema, err := database.ExtractSchema(runtime, engineName)
	if err != nil {
		return fmt.Errorf("schema extraction failed: %w", err)
	}

	// ── 5. Build prompt ─────────────────────────────────────────────────────
	userPrompt := fmt.Sprintf(
		"Generate exactly %d realistic, chaotic synthetic INSERT statements for the following database schema:\n\n%s",
		synthRecords, schema)

	// ── 6. Dry-run gate ─────────────────────────────────────────────────────
	if DryRun {
		if outputJSON {
			b, _ := json.MarshalIndent(map[string]interface{}{
				"engine":            engineName,
				"provider":          provider.Name,
				"provider_source":   provider.Source,
				"model":             resolveModel(provider),
				"records_requested": synthRecords,
				"schema_length":     len(schema),
				"dry_run":           true,
			}, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		fmt.Printf("\n── Schema (%d bytes) ──\n%s\n", len(schema), schema)
		fmt.Printf("\n── AI Provider ──\n  Name:   %s (%s)\n  Model:  %s\n", provider.Name, provider.Source, resolveModel(provider))
		fmt.Printf("\n── Prompt ──\n%s\n", userPrompt)
		fmt.Println("\n[dry-run] Would send the above prompt and pipe generated SQL into the database.")
		return nil
	}

	// ── 7. Confirmation ─────────────────────────────────────────────────────
	if !NonInteractive {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Generate %d synthetic records for %s?", synthRecords, engineName)).
					Description(fmt.Sprintf(
						"Provider: %s (%s)\nModel:    %s",
						provider.Name,
						provider.Source,
						tui.StyleMuted.Render(resolveModel(provider)),
					)).
					Affirmative("Yes, generate").
					Negative("Cancel").
					Value(&confirmed),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// ── 8. Generate completion ──────────────────────────────────────────────
	model := synthModel
	if !outputJSON {
		fmt.Printf("🤖 Generating %d records via %s...\n", synthRecords, provider.Name)
	}

	rawSQL, err := ai.GenerateCompletion(provider, model, synthesizeSystemPrompt, userPrompt)
	if err != nil {
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBSynthLLMFailed,
			Message:  fmt.Sprintf("LLM generation failed via %s", provider.Name),
			Err:      err,
		}
	}

	// ── 9. Sanitize ─────────────────────────────────────────────────────────
	sanitizedSQL := database.SanitizeLLMSQL(rawSQL)
	if sanitizedSQL == "" {
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBSynthLLMFailed,
			Message:  "LLM returned empty response",
		}
	}

	// ── 10. Execute ─────────────────────────────────────────────────────────
	if !outputJSON {
		fmt.Printf("💾 Piping %d bytes of SQL into %s...\n", len(sanitizedSQL), engineName)
	}

	if err := database.PipeSQL(runtime, engineName, sanitizedSQL); err != nil {
		if !outputJSON {
			fmt.Printf("\n── Generated SQL (for debugging) ──\n%s\n", sanitizedSQL)
		}
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBSynthSQLFailed,
			Message:  fmt.Sprintf("Generated SQL failed to execute against %s", engineName),
			Err:      err,
		}
	}

	// ── 11. Success ─────────────────────────────────────────────────────────
	if outputJSON {
		b, _ := json.MarshalIndent(map[string]interface{}{
			"engine":            engineName,
			"provider":          provider.Name,
			"provider_source":   provider.Source,
			"model":             resolveModel(provider),
			"records_requested": synthRecords,
			"sql_bytes":         len(sanitizedSQL),
			"success":           true,
		}, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Printf("\n%s Successfully generated synthetic data for %s via %s.\n", tui.IconDone, engineName, provider.Name)
	}

	return nil
}

// resolveModel returns the effective model name for display purposes.
func resolveModel(provider *ai.AIProvider) string {
	if synthModel != "" {
		return synthModel
	}
	if provider.DefaultModel != "" {
		return provider.DefaultModel
	}
	return "(server default)"
}
