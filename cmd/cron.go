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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/cron"
	"github.com/VitruvianSoftware/devx/internal/envvault"
)

var cronCmd = &cobra.Command{
	Use:     "cron",
	GroupID: "orchestration",
	Short:   "Run and inspect project-defined cron jobs locally",
	Long: `Manage cron jobs declared in your devx.yaml's 'cron:' block.

Production cron jobs (the things that run nightly on your servers or as
Kubernetes CronJobs) are notoriously hard to test locally — you either wait
hours for the schedule to fire, or you cobble together a one-off invocation
with the right env vars. 'devx cron' makes the local-test loop trivial:

  devx cron list           Show every job declared in devx.yaml.
  devx cron run <name>     Execute one job now, with the project's vault
                           secrets and per-job env merged in.

The 'schedule' field is informational only in this version — there is no
long-running scheduler. The point is local invocation parity with prod.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(cronCmd)
}

// loadCronJobs reads devx.yaml (including profiles + includes) and converts
// the raw DevxConfigCron entries into internal/cron Job structs, validating
// each. Returns a clear error if any job is malformed so the user finds out
// at `list` / `run` time rather than mid-execution.
func loadCronJobs() ([]cron.Job, error) {
	yamlPath, err := mustFindDevxConfig()
	if err != nil {
		return nil, err
	}
	cfg, err := resolveConfig(yamlPath, "")
	if err != nil {
		return nil, fmt.Errorf("could not read devx.yaml: %w", err)
	}

	jobs := make([]cron.Job, 0, len(cfg.Cron))
	for i, raw := range cfg.Cron {
		timeout, err := cron.ParseTimeout(raw.Timeout)
		if err != nil {
			return nil, fmt.Errorf("cron job index %d (%q): %w", i, raw.Name, err)
		}
		job := cron.Job{
			Name:        raw.Name,
			Schedule:    raw.Schedule,
			Command:     raw.Command,
			Description: raw.Description,
			Timeout:     timeout,
			Env:         raw.Env,
		}
		if err := job.Validate(); err != nil {
			return nil, fmt.Errorf("cron job index %d: %w", i, err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// loadVaultSecretsForCron pulls the same vault secrets `devx shell` would
// inject. Best-effort: a vault fetch failure logs a warning to stderr and
// returns an empty map so a cron job can still run with whatever the host
// env already provides.
func loadVaultSecretsForCron() map[string]string {
	secrets, source, err := loadAvailableSecrets()
	if err != nil {
		_, _ = fmt.Fprintf(rootCmd.ErrOrStderr(),
			"⚠️  Vault fetch failed (continuing without secrets): %v\n", err)
		return map[string]string{}
	}
	if len(secrets) > 0 && !outputJSON {
		fmt.Printf("🔒 Injected %d secrets from %s\n", len(secrets), source)
	}
	return secrets
}

// _ keeps the envvault import live in case future cron code wants to call
// it directly (e.g. for per-job vault selection).
var _ = envvault.PullAll
