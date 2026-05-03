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
	"sort"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/envvault"
	"github.com/VitruvianSoftware/devx/internal/tui"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var validateSchemaFile string

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate environment variables against a schema before starting the app",
	Long: `Cross-references the keys declared in your schema file (.env.example by default)
against the secrets actually available from your vault sources (devx.yaml) or .env file.

Catches missing or empty keys before they cause cryptic application crashes at runtime.

Schema file resolution order:
  1. --schema flag (explicit override)
  2. .env.example  (conventional)
  3. .env.schema   (alternative convention)`,
	RunE: runConfigValidate,
}

func init() {
	configValidateCmd.Flags().StringVar(&validateSchemaFile, "schema", "",
		"Path to schema file listing required keys (default: .env.example or .env.schema)")
	configCmd.AddCommand(configValidateCmd)
}

type validationResult struct {
	Key    string `json:"key"`
	Status string `json:"status"` // "ok", "missing", "empty"
	Source string `json:"source,omitempty"`
}

func runConfigValidate(_ *cobra.Command, _ []string) error {
	// 1. Resolve schema file
	schemaPath, err := resolveSchemaFile(validateSchemaFile)
	if err != nil {
		return err
	}
	fmt.Printf("📋 Schema: %s\n", schemaPath)

	// 2. Load required keys from schema
	requiredKeys, err := loadSchemaKeys(schemaPath)
	if err != nil {
		return fmt.Errorf("could not read schema file %q: %w", schemaPath, err)
	}
	if len(requiredKeys) == 0 {
		fmt.Println("No keys declared in schema file — nothing to validate.")
		return nil
	}

	// 3. Load available secrets from devx.yaml vault sources (or .env fallback)
	available, source, err := loadAvailableSecrets()
	if err != nil {
		return fmt.Errorf("could not load secrets: %w", err)
	}
	fmt.Printf("🔑 Secret source: %s\n\n", source)

	// 4. Compare
	var results []validationResult
	missing := 0
	empty := 0

	for _, key := range requiredKeys {
		val, exists := available[key]
		switch {
		case !exists:
			results = append(results, validationResult{Key: key, Status: "missing"})
			missing++
		case strings.TrimSpace(val) == "":
			results = append(results, validationResult{Key: key, Status: "empty"})
			empty++
		default:
			results = append(results, validationResult{Key: key, Status: "ok", Source: source})
		}
	}

	if outputJSON {
		b, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(b))
		if missing+empty > 0 {
			os.Exit(1)
		}
		return nil
	}

	// Pretty print results
	for _, r := range results {
		switch r.Status {
		case "ok":
			fmt.Printf("  %s %s\n", tui.StyleDetailDone.Render("✓"), tui.StyleLabel.Render(r.Key))
		case "missing":
			fmt.Printf("  %s %s  %s\n",
				tui.StyleDetailError.Render("✗"),
				tui.StyleLabel.Render(r.Key),
				tui.StyleMuted.Render("(missing — not found in any vault source)"),
			)
		case "empty":
			fmt.Printf("  %s %s  %s\n",
				tui.StyleDetailRunning.Render("⚠"),
				tui.StyleLabel.Render(r.Key),
				tui.StyleMuted.Render("(present but empty)"),
			)
		}
	}

	fmt.Println()

	if missing+empty > 0 {
		fmt.Printf("  %d of %d keys failed validation", missing+empty, len(requiredKeys))
		if empty > 0 {
			fmt.Printf(" (%d missing, %d empty)", missing, empty)
		}
		fmt.Println()
		fmt.Println()
		fmt.Println("  Run 'devx config pull' to sync secrets from your vault.")
		return fmt.Errorf("environment validation failed: %d missing, %d empty", missing, empty)
	}

	fmt.Printf("  %s All %d required keys are present.\n",
		tui.StyleDetailDone.Render("✓"),
		len(requiredKeys),
	)
	return nil
}

// resolveSchemaFile finds the schema file to validate against.
func resolveSchemaFile(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("schema file %q not found", explicit)
		}
		return explicit, nil
	}
	for _, candidate := range []string{".env.example", ".env.schema"} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no schema file found — create .env.example listing required keys, or use --schema <path>")
}

// loadSchemaKeys reads a .env.example-style file and returns only the key names.
// Values in the schema file are treated as documentation/examples and ignored.
func loadSchemaKeys(path string) ([]string, error) {
	parsed, err := godotenv.Read(path)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(parsed))
	for k := range parsed {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// loadAvailableSecrets fetches the actual secret values from devx.yaml vault
// sources, falling back to .env if devx.yaml is absent.
// Idea 44: uses resolveConfig so env sources from included projects are merged.
func loadAvailableSecrets() (map[string]string, string, error) {
	if yamlPath, err := findDevxConfig(); err == nil {
		if cfg, err := resolveConfig(yamlPath, ""); err == nil && len(cfg.Env) > 0 {
			secrets, serr := envvault.PullAll(cfg.Env)
			if serr != nil {
				return nil, "", serr
			}
			return secrets, fmt.Sprintf("devx.yaml (%s)", strings.Join(cfg.Env, ", ")), nil
		}
	}

	// Fallback: plain .env
	if _, serr := os.Stat(".env"); serr == nil {
		secrets, rerr := envvault.PullAll([]string{"file://.env"})
		if rerr != nil {
			return nil, "", rerr
		}
		return secrets, ".env", nil
	}

	return map[string]string{}, "(no sources found)", nil
}
