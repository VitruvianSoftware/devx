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
	"regexp"
	"strings"
)

// SynthesizableEngines returns the list of engines that support DDL extraction
// for AI-driven synthetic data generation.
func SynthesizableEngines() []string {
	return []string{"postgres", "mysql"}
}

// IsSynthesizable checks if an engine supports schema extraction.
func IsSynthesizable(engine string) bool {
	for _, e := range SynthesizableEngines() {
		if e == engine {
			return true
		}
	}
	return false
}

// ExtractSchema extracts the DDL (schema-only) from a running devx database container.
// For PostgreSQL, it uses pg_dump -s. For MySQL, it uses mysqldump --no-data.
func ExtractSchema(runtime, engineName string) (string, error) {
	containerName := fmt.Sprintf("devx-db-%s", engineName)

	var cmd *exec.Cmd
	switch engineName {
	case "postgres":
		cmd = exec.Command(runtime, "exec", "-i", containerName,
			"pg_dump", "-s", "-U", "devx", "devx")
	case "mysql":
		cmd = exec.Command(runtime, "exec", "-i", containerName,
			"mysqldump", "--no-data", "-u", "devx", "-pdevx", "devx")
	default:
		return "", fmt.Errorf("engine %q does not support schema extraction — supported: %s",
			engineName, strings.Join(SynthesizableEngines(), ", "))
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("schema extraction failed for %s: %w\nOutput: %s", engineName, err, string(out))
	}

	schema := strings.TrimSpace(string(out))
	if schema == "" {
		return "", fmt.Errorf("no schema found in %s — run your migrations first", engineName)
	}

	return schema, nil
}

// markdownCodeBlockRe matches ```sql ... ``` or ``` ... ``` blocks.
var markdownCodeBlockRe = regexp.MustCompile("(?s)```(?:sql)?\\s*\n?(.*?)\\s*```")

// SanitizeLLMSQL strips markdown code block wrappers from LLM output and
// ensures the SQL is wrapped in a transaction if not already.
func SanitizeLLMSQL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// If the response contains markdown code blocks, extract the SQL from them
	if matches := markdownCodeBlockRe.FindAllStringSubmatch(raw, -1); len(matches) > 0 {
		var parts []string
		for _, m := range matches {
			parts = append(parts, strings.TrimSpace(m[1]))
		}
		raw = strings.Join(parts, "\n")
	}

	// Clean up any remaining backticks that might be loose
	raw = strings.ReplaceAll(raw, "```sql", "")
	raw = strings.ReplaceAll(raw, "```", "")
	raw = strings.TrimSpace(raw)

	// Wrap in transaction if not already
	upper := strings.ToUpper(raw)
	if !strings.HasPrefix(upper, "BEGIN") {
		raw = "BEGIN;\n" + raw + "\nCOMMIT;"
	}

	return raw
}

// PipeSQL pipes raw SQL into a running devx database container via stdin.
func PipeSQL(runtime, engineName, sql string) error {
	containerName := fmt.Sprintf("devx-db-%s", engineName)

	var cmd *exec.Cmd
	switch engineName {
	case "postgres":
		cmd = exec.Command(runtime, "exec", "-i", containerName,
			"psql", "-U", "devx", "-d", "devx")
	case "mysql":
		cmd = exec.Command(runtime, "exec", "-i", containerName,
			"mysql", "-u", "devx", "-pdevx", "devx")
	default:
		return fmt.Errorf("engine %q does not support SQL piping", engineName)
	}

	cmd.Stdin = strings.NewReader(sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("SQL execution failed: %w\nOutput: %s", err, string(out))
	}

	return nil
}
