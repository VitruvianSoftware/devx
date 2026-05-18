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

// Package cron implements one-shot execution of project-defined scheduled
// tasks declared in devx.yaml's `cron:` block. The schedule expression is
// informational in this version — `devx cron run <name>` fires the job
// immediately with the same env + secrets a `devx shell` session would get,
// so developers can verify the job's behavior without waiting for the
// production cron schedule to roll around.
package cron

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Job is the in-memory shape of a single cron entry, normalized from the
// devx.yaml DevxConfigCron struct. The cmd package converts to this so the
// cron package has no dependency on cmd.
type Job struct {
	Name        string
	Schedule    string
	Command     []string
	Description string
	Timeout     time.Duration
	Env         map[string]string
}

// Validate checks the invariants required to actually execute a job.
// Returns nil if the job is runnable.
func (j Job) Validate() error {
	if strings.TrimSpace(j.Name) == "" {
		return errors.New("cron job missing name")
	}
	if len(j.Command) == 0 {
		return fmt.Errorf("cron job %q has no command", j.Name)
	}
	for _, arg := range j.Command {
		if arg == "" {
			return fmt.Errorf("cron job %q has an empty command element", j.Name)
		}
	}
	return nil
}

// ExecuteConfig holds the runtime parameters for a single Execute call.
type ExecuteConfig struct {
	Job     Job
	Secrets map[string]string // merged with Job.Env (Job.Env wins on conflict)
	WorkDir string            // working directory for the subprocess; "" means inherit
	// Stdout / Stderr default to os.Stdout / os.Stderr when nil. Injectable
	// for tests so we can assert on captured output.
	Stdout *os.File
	Stderr *os.File
}

// Result captures the outcome of a single run for both human and JSON
// reporting. ExitCode is -1 if the process failed to start.
type Result struct {
	Job        string        `json:"job"`
	StartedAt  time.Time     `json:"started_at"`
	Duration   time.Duration `json:"duration"`
	DurationMs int64         `json:"duration_ms"`
	ExitCode   int           `json:"exit_code"`
	TimedOut   bool          `json:"timed_out,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// Execute runs the job once with the merged env, streaming stdout/stderr to
// the configured writers. Honors the job's Timeout if set; honors ctx
// cancellation always. Returns a populated Result even on error so callers
// can render it uniformly.
func Execute(ctx context.Context, cfg ExecuteConfig) (Result, error) {
	res := Result{Job: cfg.Job.Name, StartedAt: time.Now(), ExitCode: -1}

	if err := cfg.Job.Validate(); err != nil {
		res.Error = err.Error()
		return res, err
	}

	runCtx := ctx
	if cfg.Job.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, cfg.Job.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(runCtx, cfg.Job.Command[0], cfg.Job.Command[1:]...)
	cmd.Env = mergeEnv(os.Environ(), cfg.Secrets, cfg.Job.Env)
	if cfg.WorkDir != "" {
		cmd.Dir = cfg.WorkDir
	}
	cmd.Stdout = orStdout(cfg.Stdout)
	cmd.Stderr = orStderr(cfg.Stderr)

	err := cmd.Run()
	res.Duration = time.Since(res.StartedAt)
	res.DurationMs = res.Duration.Milliseconds()

	if err == nil {
		res.ExitCode = 0
		return res, nil
	}

	// Distinguish timeout (a devx-level decision) from a normal non-zero
	// exit (which is the job's own signal).
	if runCtx.Err() == context.DeadlineExceeded {
		res.TimedOut = true
		res.Error = fmt.Sprintf("job timed out after %s", cfg.Job.Timeout)
		return res, fmt.Errorf("cron job %q timed out after %s", cfg.Job.Name, cfg.Job.Timeout)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		res.Error = fmt.Sprintf("exit code %d", res.ExitCode)
		return res, fmt.Errorf("cron job %q failed: exit code %d", cfg.Job.Name, res.ExitCode)
	}

	res.Error = err.Error()
	return res, fmt.Errorf("cron job %q failed: %w", cfg.Job.Name, err)
}

// ParseTimeout converts the YAML-string timeout to a duration. An empty
// string yields zero (meaning no timeout). Surfaced separately so the CLI
// layer can validate timeouts at load time rather than at execute time.
func ParseTimeout(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout %q (expected Go duration like '5m', '30s'): %w", s, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid timeout %q (must be positive)", s)
	}
	return d, nil
}

// SortedNames returns job names in alphabetical order — used by `devx cron
// list` for stable output and by error messages that hint at valid names.
func SortedNames(jobs []Job) []string {
	out := make([]string, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, j.Name)
	}
	sort.Strings(out)
	return out
}

// FindByName performs a case-sensitive lookup in the job list. Returns
// (Job, true) on hit, (zero, false) on miss.
func FindByName(jobs []Job, name string) (Job, bool) {
	for _, j := range jobs {
		if j.Name == name {
			return j, true
		}
	}
	return Job{}, false
}

// mergeEnv builds the final env slice for exec.Cmd. Order matters: later
// maps override earlier ones, mirroring shell precedence so a per-job
// `env:` block beats a vault secret with the same key.
func mergeEnv(base []string, layers ...map[string]string) []string {
	merged := make(map[string]string, len(base))
	for _, kv := range base {
		if i := strings.IndexByte(kv, '='); i > 0 {
			merged[kv[:i]] = kv[i+1:]
		}
	}
	for _, layer := range layers {
		for k, v := range layer {
			merged[k] = v
		}
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	sort.Strings(out) // deterministic for tests
	return out
}

func orStdout(f *os.File) *os.File {
	if f == nil {
		return os.Stdout
	}
	return f
}

func orStderr(f *os.File) *os.File {
	if f == nil {
		return os.Stderr
	}
	return f
}
