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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/cron"
)

var cronRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Execute a single cron job now, with the project's secrets and env",
	Long: `Run the named cron job immediately (independent of its schedule).
Pulls vault secrets the same way 'devx shell' does, merges per-job env vars
on top, and streams stdout/stderr to your terminal.

Returns a non-zero exit code if the job fails, times out, or is unknown —
so chaining works in CI and pre-push hooks:

  devx cron run nightly-cleanup && echo "✓ cleanup OK"

The job's exact exit code is reported in the human / JSON output so you can
inspect it; the process exit itself is the standard cobra exit-1-on-error
(use --json + jq if you need the precise code programmatically).

Use --dry-run to see what would execute without actually running it.`,
	Args: cobra.ExactArgs(1),
	RunE: runCronRun,
}

func init() {
	cronCmd.AddCommand(cronRunCmd)
}

func runCronRun(_ *cobra.Command, args []string) error {
	name := args[0]

	jobs, err := loadCronJobs()
	if err != nil {
		return err
	}
	job, ok := cron.FindByName(jobs, name)
	if !ok {
		known := cron.SortedNames(jobs)
		if len(known) == 0 {
			return fmt.Errorf("no cron jobs are declared in devx.yaml")
		}
		return fmt.Errorf("no cron job named %q — known jobs: %s",
			name, strings.Join(known, ", "))
	}

	// Cancel cleanly on Ctrl+C so the subprocess gets a SIGTERM rather than
	// being orphaned.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if DryRun {
		return reportDryRun(job)
	}

	if !outputJSON {
		fmt.Printf("🕐 Running cron job: %s\n", job.Name)
		if job.Schedule != "" {
			fmt.Printf("   Schedule:   %s  (informational; one-shot run)\n", job.Schedule)
		}
		if job.Description != "" {
			fmt.Printf("   Purpose:    %s\n", job.Description)
		}
		fmt.Printf("   Command:    %s\n", strings.Join(job.Command, " "))
		if job.Timeout > 0 {
			fmt.Printf("   Timeout:    %s\n", job.Timeout)
		}
	}

	secrets := loadVaultSecretsForCron()

	res, runErr := cron.Execute(ctx, cron.ExecuteConfig{
		Job:     job,
		Secrets: secrets,
	})

	if outputJSON {
		_ = json.NewEncoder(os.Stdout).Encode(res)
	} else {
		fmt.Println()
		switch {
		case res.TimedOut:
			fmt.Printf("⏱️  Timed out after %s\n", res.Duration)
		case runErr == nil:
			fmt.Printf("✅ Completed in %s (exit 0)\n", res.Duration)
		default:
			fmt.Printf("❌ Failed after %s (exit %d)\n", res.Duration, res.ExitCode)
		}
	}

	return runErr
}

// reportDryRun prints what would execute without actually running it.
// Honors --json for AI agent consumption.
func reportDryRun(job cron.Job) error {
	if outputJSON {
		payload := map[string]any{
			"dry_run":     true,
			"name":        job.Name,
			"schedule":    job.Schedule,
			"command":     job.Command,
			"description": job.Description,
		}
		if job.Timeout > 0 {
			payload["timeout"] = job.Timeout.String()
		}
		return json.NewEncoder(os.Stdout).Encode(payload)
	}
	fmt.Printf("🧪 [dry-run] Would execute cron job: %s\n", job.Name)
	if job.Schedule != "" {
		fmt.Printf("   Schedule: %s\n", job.Schedule)
	}
	fmt.Printf("   Command:  %s\n", strings.Join(job.Command, " "))
	if job.Timeout > 0 {
		fmt.Printf("   Timeout:  %s\n", job.Timeout)
	}
	if len(job.Env) > 0 {
		fmt.Printf("   Env keys: %s\n", strings.Join(sortedKeys(job.Env), ", "))
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// keep import-free sort by inline bubble — list is tiny
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
