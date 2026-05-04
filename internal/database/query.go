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

package database

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Canned Queries ──────────────────────────────────────────────────────────

// CannedQuery defines a pre-built diagnostic query.
type CannedQuery struct {
	Name        string // Short name for the query (e.g., "sizes")
	Description string // Human-readable description
	SQL         map[string]string // Engine-specific SQL (keyed by engine name)
}

// CannedQueries is the registry of built-in diagnostic queries.
var CannedQueries = map[string]CannedQuery{
	"sizes": {
		Name:        "sizes",
		Description: "Table sizes and row counts",
		SQL: map[string]string{
			"postgres": `SELECT
	schemaname || '.' || relname AS "table",
	pg_size_pretty(pg_total_relation_size(relid)) AS "total_size",
	n_live_tup AS "estimated_rows"
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(relid) DESC;`,
			"mysql": `SELECT
	table_name AS 'table',
	CONCAT(ROUND(data_length / 1024, 2), ' KB') AS 'data_size',
	table_rows AS 'estimated_rows'
FROM information_schema.tables
WHERE table_schema = DATABASE()
ORDER BY data_length DESC;`,
		},
	},
	"recent": {
		Name:        "recent",
		Description: "Last 10 rows from each user table",
		SQL: map[string]string{
			// For "recent", we need dynamic SQL — we'll handle this specially
			// by listing tables and querying each one
			"postgres": `SELECT table_name FROM information_schema.tables
WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
ORDER BY table_name;`,
			"mysql": `SELECT table_name FROM information_schema.tables
WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
ORDER BY table_name;`,
		},
	},
	"missing-indexes": {
		Name:        "missing-indexes",
		Description: "Tables with no non-primary indexes (potential performance risk)",
		SQL: map[string]string{
			"postgres": `SELECT
	t.tablename AS "table",
	COALESCE(idx.index_count, 0) AS "index_count",
	CASE WHEN COALESCE(idx.index_count, 0) = 0
		THEN '⚠️  No indexes'
		ELSE '✓'
	END AS "status"
FROM pg_tables t
LEFT JOIN (
	SELECT tablename, COUNT(*) AS index_count
	FROM pg_indexes
	WHERE schemaname = 'public'
	GROUP BY tablename
) idx ON t.tablename = idx.tablename
WHERE t.schemaname = 'public'
ORDER BY COALESCE(idx.index_count, 0) ASC, t.tablename;`,
			"mysql": `SELECT
	t.table_name AS 'table',
	COALESCE(idx.index_count, 0) AS 'index_count',
	CASE WHEN COALESCE(idx.index_count, 0) = 0
		THEN 'No indexes'
		ELSE 'OK'
	END AS 'status'
FROM information_schema.tables t
LEFT JOIN (
	SELECT table_name, COUNT(DISTINCT index_name) AS index_count
	FROM information_schema.statistics
	WHERE table_schema = DATABASE()
	GROUP BY table_name
) idx ON t.table_name = idx.table_name
WHERE t.table_schema = DATABASE()
ORDER BY COALESCE(idx.index_count, 0) ASC, t.table_name;`,
		},
	},
	"nulls": {
		Name:        "nulls",
		Description: "Columns with high NULL ratios for a specific table",
		SQL: map[string]string{
			// %s is replaced with the table name
			"postgres": `SELECT
	attname AS "column",
	n_live_tup AS "total_rows",
	null_frac AS "null_ratio",
	CASE
		WHEN null_frac > 0.5 THEN '⚠️  >50%% NULLs'
		WHEN null_frac > 0.1 THEN '~' || ROUND(null_frac * 100) || '%%'
		ELSE '✓ low'
	END AS "status"
FROM pg_stats
WHERE schemaname = 'public' AND tablename = '%s'
ORDER BY null_frac DESC;`,
			"mysql": `SELECT
	column_name AS 'column',
	is_nullable AS 'nullable',
	column_type AS 'type',
	column_default AS 'default_value'
FROM information_schema.columns
WHERE table_schema = DATABASE() AND table_name = '%s'
ORDER BY ordinal_position;`,
		},
	},
}

// ─── Query Execution ─────────────────────────────────────────────────────────

// QueryResult holds the output of an executed SQL query.
type QueryResult struct {
	Headers []string   // Column names
	Rows    [][]string // Row data
	SQL     string     // The SQL that was executed
}

// ExecuteReadOnlyQuery runs a SQL query inside a read-only transaction against
// a running devx database container and returns the parsed results.
func ExecuteReadOnlyQuery(runtime, engineName, sql string) (*QueryResult, error) {
	return executeQuery(runtime, engineName, sql, true)
}

// ExecuteQuery runs a SQL query against a running devx database container.
// If readOnly is true, the query is wrapped in a read-only transaction.
func ExecuteQuery(runtime, engineName, sql string, readOnly bool) (*QueryResult, error) {
	return executeQuery(runtime, engineName, sql, readOnly)
}

func executeQuery(runtime, engineName, sql string, readOnly bool) (*QueryResult, error) {
	containerName := fmt.Sprintf("devx-db-%s", engineName)

	var cmd *exec.Cmd
	var wrappedSQL string

	switch engineName {
	case "postgres":
		if readOnly {
			wrappedSQL = fmt.Sprintf("BEGIN TRANSACTION READ ONLY;\n%s\nCOMMIT;", sql)
		} else {
			wrappedSQL = sql
		}
		// Use \x off for tabular output, and add column headers with aligned output
		cmd = exec.Command(runtime, "exec", "-i", containerName,
			"psql", "-U", "devx", "-d", "devx",
			"--no-align", "--field-separator=\t", "--tuples-only=off",
			"-c", wrappedSQL)
	case "mysql":
		if readOnly {
			wrappedSQL = fmt.Sprintf("SET TRANSACTION READ ONLY;\nSTART TRANSACTION;\n%s\nCOMMIT;", sql)
		} else {
			wrappedSQL = sql
		}
		cmd = exec.Command(runtime, "exec", "-i", containerName,
			"mysql", "-u", "devx", "-pdevx", "devx",
			"-e", wrappedSQL)
	default:
		return nil, fmt.Errorf("engine %q does not support queries — supported: postgres, mysql", engineName)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(string(out))
		// Check for read-only violation
		if readOnly && (strings.Contains(output, "cannot execute") || strings.Contains(output, "READ ONLY")) {
			return nil, fmt.Errorf("query attempted a write operation — use --allow-writes to enable mutations:\n%s", output)
		}
		return nil, fmt.Errorf("query execution failed: %w\nOutput: %s", err, output)
	}

	return parseQueryOutput(engineName, string(out), sql), nil
}

// parseQueryOutput parses tab-separated query output into structured results.
func parseQueryOutput(engineName, output, sql string) *QueryResult {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	result := &QueryResult{SQL: sql}

	if len(lines) == 0 {
		return result
	}

	// Filter out psql noise (SET, BEGIN, COMMIT, row count lines)
	var dataLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "SET" || trimmed == "BEGIN" || trimmed == "COMMIT" ||
			trimmed == "START TRANSACTION" || strings.HasPrefix(trimmed, "(") {
			continue
		}
		dataLines = append(dataLines, line)
	}

	if len(dataLines) == 0 {
		return result
	}

	// First data line is headers
	result.Headers = strings.Split(dataLines[0], "\t")

	// Remaining lines are data rows
	for _, line := range dataLines[1:] {
		// Skip separator lines (psql sometimes emits them)
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+-") {
			continue
		}
		cols := strings.Split(line, "\t")
		result.Rows = append(result.Rows, cols)
	}

	return result
}

// ─── Table Rendering ─────────────────────────────────────────────────────────

var (
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#79C0FF")).
				PaddingRight(2)

	tableCellStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E6EDF3")).
			PaddingRight(2)

	tableBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#30363D"))

	tableEmptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8B949E")).
			Italic(true)
)

// RenderTable prints query results as a styled terminal table.
func RenderTable(result *QueryResult) string {
	if result == nil || len(result.Headers) == 0 {
		return tableEmptyStyle.Render("(0 rows)")
	}

	if len(result.Rows) == 0 {
		return tableEmptyStyle.Render("(0 rows)")
	}

	// Calculate column widths
	widths := make([]int, len(result.Headers))
	for i, h := range result.Headers {
		widths[i] = len(h)
	}
	for _, row := range result.Rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Clamp widths to avoid extremely wide columns
	for i := range widths {
		if widths[i] > 50 {
			widths[i] = 50
		}
		if widths[i] < 4 {
			widths[i] = 4
		}
	}

	var sb strings.Builder

	// Header row
	var headerCells []string
	for i, h := range result.Headers {
		cell := tableHeaderStyle.Width(widths[i]).Render(h)
		headerCells = append(headerCells, cell)
	}
	sb.WriteString(strings.Join(headerCells, "  "))
	sb.WriteString("\n")

	// Separator
	var sepParts []string
	for _, w := range widths {
		sepParts = append(sepParts, tableBorderStyle.Render(strings.Repeat("─", w)))
	}
	sb.WriteString(strings.Join(sepParts, "  "))
	sb.WriteString("\n")

	// Data rows
	for _, row := range result.Rows {
		var cells []string
		for i := 0; i < len(result.Headers); i++ {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			if len(val) > 50 {
				val = val[:47] + "..."
			}
			cell := tableCellStyle.Width(widths[i]).Render(val)
			cells = append(cells, cell)
		}
		sb.WriteString(strings.Join(cells, "  "))
		sb.WriteString("\n")
	}

	// Row count
	sb.WriteString(tableEmptyStyle.Render(fmt.Sprintf("(%d rows)", len(result.Rows))))

	return sb.String()
}
