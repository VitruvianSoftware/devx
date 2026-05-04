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

var (
	askRecent         bool
	askSizes          bool
	askMissingIndexes bool
	askNulls          string
	askAllowWrites    bool
	askModel          string
	askRuntime        string
)

var dbAskCmd = &cobra.Command{
	Use:   "ask <engine> [question]",
	Short: "Query your local database using natural language or built-in diagnostics",
	Long: `Ask questions about your local database in plain English, or use built-in
diagnostic queries that work without any AI provider.

Natural Language Queries (requires AI):
  devx db ask postgres "users who signed up this week"
  devx db ask mysql "show me the 5 largest tables"

Built-in Diagnostics (no AI needed):
  devx db ask postgres --sizes              Table sizes and row counts
  devx db ask postgres --recent             Last 10 rows from each table
  devx db ask postgres --missing-indexes    Tables without indexes
  devx db ask postgres --nulls users        Column NULL ratios for a table

AI Provider Priority:
  1. Local Ollama (port 11434)
  2. Local LM Studio (port 1234)
  3. OPENAI_API_KEY environment variable (cloud fallback)

Safety: All queries run inside a read-only transaction by default.
Use --allow-writes to enable mutations (with confirmation).`,
	Example: `  # Natural language query
  devx db ask postgres "users who never verified their email"

  # Preview the generated SQL without executing
  devx db ask postgres "orphaned orders" --dry-run

  # Built-in diagnostics (no AI needed)
  devx db ask postgres --sizes
  devx db ask postgres --recent
  devx db ask postgres --missing-indexes
  devx db ask postgres --nulls users

  # Allow write operations
  devx db ask postgres "delete inactive users" --allow-writes`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDBAsk,
}

func init() {
	dbAskCmd.Flags().BoolVar(&askRecent, "recent", false,
		"Show last 10 rows from each table (no AI needed)")
	dbAskCmd.Flags().BoolVar(&askSizes, "sizes", false,
		"Show table sizes and row counts (no AI needed)")
	dbAskCmd.Flags().BoolVar(&askMissingIndexes, "missing-indexes", false,
		"Show tables without non-primary indexes (no AI needed)")
	dbAskCmd.Flags().StringVar(&askNulls, "nulls", "",
		"Show column NULL ratios for a specific table (no AI needed)")
	dbAskCmd.Flags().BoolVar(&askAllowWrites, "allow-writes", false,
		"Allow the generated query to perform write operations (INSERT/UPDATE/DELETE)")
	dbAskCmd.Flags().StringVar(&askModel, "model", "",
		"Override the LLM model for query generation")
	dbAskCmd.Flags().StringVar(&askRuntime, "runtime", "podman",
		"Container runtime to use (podman, docker)")
	dbCmd.AddCommand(dbAskCmd)
}

// text2sqlSystemPrompt instructs the LLM to generate safe, precise SQL.
const text2sqlSystemPrompt = `You are an expert SQL developer. Given the database schema below, translate the user's natural language question into a single valid SQL query. Rules:
1. Output ONLY the raw SQL query. No markdown, no explanations, no code blocks, no comments.
2. Use only tables and columns that exist in the schema.
3. Prefer explicit column names over SELECT *.
4. Use appropriate JOINs when the question involves multiple tables.
5. Add LIMIT 100 unless the user explicitly asks for all results or a specific count.
6. For aggregate queries, always include meaningful column aliases.`

func runDBAsk(_ *cobra.Command, args []string) error {
	engineName := strings.ToLower(args[0])
	runtime := askRuntime

	// ── 1. Validate engine ──────────────────────────────────────────────────
	if !database.IsSynthesizable(engineName) {
		return fmt.Errorf("engine %q does not support queries — supported: %s",
			engineName, strings.Join(database.SynthesizableEngines(), ", "))
	}

	// ── 2. Verify container is running ──────────────────────────────────────
	containerName := fmt.Sprintf("devx-db-%s", engineName)
	checkCmd := exec.Command(runtime, "inspect", containerName, "--format", "{{.State.Running}}")
	if out, err := checkCmd.Output(); err != nil || strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("the local %s database is not running — start it with 'devx db spawn %s'",
			engineName, engineName)
	}

	// ── 3. Dispatch: canned query or natural language ───────────────────────
	if askSizes {
		return runCannedQuery(runtime, engineName, "sizes")
	}
	if askMissingIndexes {
		return runCannedQuery(runtime, engineName, "missing-indexes")
	}
	if askNulls != "" {
		return runCannedQueryWithArg(runtime, engineName, "nulls", askNulls)
	}
	if askRecent {
		return runRecentQuery(runtime, engineName)
	}

	// Natural language query requires a question argument
	if len(args) < 2 {
		return fmt.Errorf("please provide a question in quotes, or use a built-in flag (--sizes, --recent, --missing-indexes, --nulls)")
	}

	question := args[1]
	return runNaturalLanguageQuery(runtime, engineName, question)
}

// ─── Canned Queries ──────────────────────────────────────────────────────────

func runCannedQuery(runtime, engineName, queryName string) error {
	canned, ok := database.CannedQueries[queryName]
	if !ok {
		return fmt.Errorf("unknown canned query: %s", queryName)
	}

	sql, ok := canned.SQL[engineName]
	if !ok {
		return fmt.Errorf("canned query %q is not supported for engine %s", queryName, engineName)
	}

	if !outputJSON {
		fmt.Printf("📊 %s — %s\n\n", canned.Name, canned.Description)
	}

	if DryRun {
		if outputJSON {
			b, _ := json.MarshalIndent(map[string]interface{}{
				"engine":   engineName,
				"query":    queryName,
				"sql":      sql,
				"dry_run":  true,
			}, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Printf("SQL:\n%s\n", sql)
		}
		return nil
	}

	result, err := database.ExecuteReadOnlyQuery(runtime, engineName, sql)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return renderResult(result, engineName)
}

func runCannedQueryWithArg(runtime, engineName, queryName, arg string) error {
	canned, ok := database.CannedQueries[queryName]
	if !ok {
		return fmt.Errorf("unknown canned query: %s", queryName)
	}

	sqlTemplate, ok := canned.SQL[engineName]
	if !ok {
		return fmt.Errorf("canned query %q is not supported for engine %s", queryName, engineName)
	}

	// Sanitize table name (basic SQL injection prevention for canned queries)
	safeName := sanitizeIdentifier(arg)
	sql := fmt.Sprintf(sqlTemplate, safeName)

	if !outputJSON {
		fmt.Printf("📊 %s — %s (table: %s)\n\n", canned.Name, canned.Description, safeName)
	}

	if DryRun {
		if outputJSON {
			b, _ := json.MarshalIndent(map[string]interface{}{
				"engine":   engineName,
				"query":    queryName,
				"table":    safeName,
				"sql":      sql,
				"dry_run":  true,
			}, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Printf("SQL:\n%s\n", sql)
		}
		return nil
	}

	result, err := database.ExecuteReadOnlyQuery(runtime, engineName, sql)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return renderResult(result, engineName)
}

func runRecentQuery(runtime, engineName string) error {
	if !outputJSON {
		fmt.Printf("📊 recent — Last 10 rows from each table\n\n")
	}

	if DryRun {
		fmt.Println("[dry-run] Would query each user table for its last 10 rows")
		return nil
	}

	// First, get the list of tables
	canned := database.CannedQueries["recent"]
	listSQL, ok := canned.SQL[engineName]
	if !ok {
		return fmt.Errorf("recent query is not supported for engine %s", engineName)
	}

	result, err := database.ExecuteReadOnlyQuery(runtime, engineName, listSQL)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}

	if len(result.Rows) == 0 {
		fmt.Println(tui.StyleMuted.Render("(no tables found — run your migrations first)"))
		return nil
	}

	// Query each table
	for _, row := range result.Rows {
		if len(row) == 0 {
			continue
		}
		tableName := strings.TrimSpace(row[0])
		if tableName == "" {
			continue
		}

		var sql string
		switch engineName {
		case "postgres":
			sql = fmt.Sprintf("SELECT * FROM %s ORDER BY ctid DESC LIMIT 10;", sanitizeIdentifier(tableName))
		case "mysql":
			sql = fmt.Sprintf("SELECT * FROM %s LIMIT 10;", sanitizeIdentifier(tableName))
		}

		if !outputJSON {
			fmt.Printf("── %s ──\n", tableName)
		}

		tableResult, err := database.ExecuteReadOnlyQuery(runtime, engineName, sql)
		if err != nil {
			fmt.Printf("  (error querying %s: %v)\n\n", tableName, err)
			continue
		}

		if outputJSON {
			b, _ := json.MarshalIndent(map[string]interface{}{
				"table":   tableName,
				"headers": tableResult.Headers,
				"rows":    tableResult.Rows,
			}, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Println(database.RenderTable(tableResult))
			fmt.Println()
		}
	}

	return nil
}

// ─── Natural Language Queries ────────────────────────────────────────────────

func runNaturalLanguageQuery(runtime, engineName, question string) error {
	// ── 1. Discover AI provider ─────────────────────────────────────────────
	provider := ai.DiscoverAIProvider()
	if provider == nil {
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBAskNoAI,
			Message: `No AI provider found for natural language queries.
  Start Ollama (ollama serve), LM Studio, or export OPENAI_API_KEY.
  Alternatively, use built-in queries: --sizes, --recent, --missing-indexes, --nulls`,
		}
	}

	// ── 2. Extract schema ───────────────────────────────────────────────────
	if !outputJSON {
		fmt.Printf("📋 Extracting schema from %s...\n", engineName)
	}

	schema, err := database.ExtractSchema(runtime, engineName)
	if err != nil {
		return fmt.Errorf("schema extraction failed: %w", err)
	}

	// ── 3. Build prompt ─────────────────────────────────────────────────────
	userPrompt := fmt.Sprintf(
		"Database schema:\n\n%s\n\nQuestion: %s",
		schema, question)

	// ── 4. Dry-run gate ─────────────────────────────────────────────────────
	if DryRun {
		if outputJSON {
			b, _ := json.MarshalIndent(map[string]interface{}{
				"engine":          engineName,
				"question":        question,
				"provider":        provider.Name,
				"provider_source": provider.Source,
				"model":           askResolveModel(provider),
				"schema_length":   len(schema),
				"dry_run":         true,
			}, "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Printf("  Provider:  %s (%s)\n", provider.Name, provider.Source)
			fmt.Printf("  Model:     %s\n", askResolveModel(provider))
			fmt.Printf("  Question:  %s\n", question)
			fmt.Printf("  Schema:    %d bytes\n\n", len(schema))
			fmt.Println("[dry-run] Would generate SQL from this question and execute against the database.")
		}
		return nil
	}

	// ── 5. Generate SQL via AI ──────────────────────────────────────────────
	if !outputJSON {
		fmt.Printf("🤖 Translating question via %s...\n", provider.Name)
	}

	model := askModel
	rawSQL, err := ai.GenerateCompletion(provider, model, text2sqlSystemPrompt, userPrompt)
	if err != nil {
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBAskQueryFailed,
			Message:  fmt.Sprintf("AI query generation failed via %s", provider.Name),
			Err:      err,
		}
	}

	// ── 6. Sanitize the generated SQL ───────────────────────────────────────
	generatedSQL := sanitizeGeneratedSQL(rawSQL)
	if generatedSQL == "" {
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBAskQueryFailed,
			Message:  "AI returned an empty response",
		}
	}

	if !outputJSON {
		fmt.Printf("\n  SQL: %s\n\n", tui.StyleMuted.Render(generatedSQL))
	}

	// ── 7. Write safety check ───────────────────────────────────────────────
	readOnly := !askAllowWrites
	if isWriteQuery(generatedSQL) && !askAllowWrites {
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBAskReadOnly,
			Message:  "The generated query contains write operations (INSERT/UPDATE/DELETE/DROP). Use --allow-writes to execute it.",
		}
	}

	if askAllowWrites && isWriteQuery(generatedSQL) && !NonInteractive {
		var confirmed bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("⚠️  This query will MODIFY your database. Continue?").
					Description(generatedSQL).
					Affirmative("Yes, execute").
					Negative("Cancel").
					Value(&confirmed),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := form.Run(); err != nil || !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// ── 8. Execute ──────────────────────────────────────────────────────────
	result, err := database.ExecuteQuery(runtime, engineName, generatedSQL, readOnly)
	if err != nil {
		if !outputJSON {
			fmt.Printf("\n── Generated SQL (for debugging) ──\n%s\n\n", generatedSQL)
		}
		return &devxerr.DevxError{
			ExitCode: devxerr.CodeDBAskQueryFailed,
			Message:  "Generated SQL failed to execute",
			Err:      err,
		}
	}

	return renderResult(result, engineName)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// renderResult outputs query results in the appropriate format.
func renderResult(result *database.QueryResult, engineName string) error {
	if outputJSON {
		b, _ := json.MarshalIndent(map[string]interface{}{
			"engine":  engineName,
			"headers": result.Headers,
			"rows":    result.Rows,
			"sql":     result.SQL,
		}, "", "  ")
		fmt.Println(string(b))
		return nil
	}

	fmt.Println(database.RenderTable(result))
	return nil
}

// sanitizeIdentifier strips dangerous characters from SQL identifiers.
func sanitizeIdentifier(name string) string {
	// Only allow alphanumeric, underscores, and dots (for schema.table)
	var safe strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '.' {
			safe.WriteRune(c)
		}
	}
	return safe.String()
}

// sanitizeGeneratedSQL cleans up LLM-generated SQL.
func sanitizeGeneratedSQL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Strip markdown code blocks
	raw = strings.TrimPrefix(raw, "```sql")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	// Remove any leading/trailing backticks
	raw = strings.Trim(raw, "`")
	raw = strings.TrimSpace(raw)

	return raw
}

// isWriteQuery checks if a SQL query contains write operations.
func isWriteQuery(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	writeKeywords := []string{"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE", "CREATE"}
	for _, kw := range writeKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

// askResolveModel returns the effective model name for display.
func askResolveModel(provider *ai.AIProvider) string {
	if askModel != "" {
		return askModel
	}
	if provider.DefaultModel != "" {
		return provider.DefaultModel
	}
	return "(server default)"
}
