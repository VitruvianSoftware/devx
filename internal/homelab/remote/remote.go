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

// Package remote provides an SSH-based executor for running commands on
// remote macOS hosts. It shells out to the system ssh binary to leverage
// the user's existing SSH config and agent.
package remote

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os/exec"
	"strings"
	"time"
)

// DefaultMaxRetries is the default number of retries for transient SSH failures.
const DefaultMaxRetries = 3

// Runner executes commands on a remote host via SSH.
type Runner struct {
	Host       string
	User       string
	Port       string
	KeyPath    string
	MaxRetries int
}

// NewRunner creates a new remote command runner for the given SSH host.
func NewRunner(host string) *Runner {
	return &Runner{
		Host:       host,
		MaxRetries: DefaultMaxRetries,
	}
}

// NewRunnerWithOpts creates a runner with custom SSH options.
func NewRunnerWithOpts(host, user, port, keyPath string) *Runner {
	r := NewRunner(host)
	r.User = user
	r.Port = port
	r.KeyPath = keyPath
	return r
}

// Run executes a command on the remote host and returns the combined output.
// It retries on transient failures with exponential backoff.
func (r *Runner) Run(ctx context.Context, command string) (string, error) {
	var lastErr error

	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			slog.Debug("retrying SSH command",
				"host", r.Host,
				"attempt", attempt+1,
				"backoff", backoff,
			)
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("ssh %s: %w (after %d attempts, last error: %v)", r.Host, ctx.Err(), attempt, lastErr)
			case <-time.After(backoff):
			}
		}

		output, err := r.runOnce(ctx, command)
		if err == nil {
			return output, nil
		}

		lastErr = err

		// Don't retry if context is cancelled or if it's a non-transient error.
		if ctx.Err() != nil {
			return "", fmt.Errorf("ssh %s: %w", r.Host, ctx.Err())
		}

		// Only retry on connection-related errors.
		errStr := err.Error()
		if !isTransient(errStr) {
			return "", err
		}

		slog.Warn("transient SSH failure",
			"host", r.Host,
			"attempt", attempt+1,
			"error", err,
		)
	}

	return "", fmt.Errorf("ssh %s: exhausted %d retries, last error: %w", r.Host, r.MaxRetries, lastErr)
}

// runOnce executes a single SSH command without retry.
func (r *Runner) runOnce(ctx context.Context, command string) (string, error) {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=accept-new",
	}

	if r.Port != "" {
		args = append(args, "-p", r.Port)
	}
	if r.KeyPath != "" {
		args = append(args, "-i", r.KeyPath)
	}

	host := r.Host
	if r.User != "" {
		host = r.User + "@" + r.Host
	}

	// Prepend Homebrew paths for non-interactive SSH sessions where
	// .zprofile/.zshrc are not loaded.
	wrappedCmd := fmt.Sprintf("export PATH=/opt/homebrew/bin:/usr/local/bin:$PATH; %s", command)
	args = append(args, host, wrappedCmd)

	slog.Debug("executing SSH command",
		"host", r.Host,
		"command", command,
	)

	cmd := exec.CommandContext(ctx, "ssh", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ssh %s: %w\nstderr: %s", r.Host, err, strings.TrimSpace(stderr.String()))
	}

	output := strings.TrimSpace(stdout.String())
	slog.Debug("SSH command completed",
		"host", r.Host,
		"output_length", len(output),
	)

	return output, nil
}

// RunShell executes a command via sh -c on the remote host.
func (r *Runner) RunShell(ctx context.Context, script string) (string, error) {
	return r.Run(ctx, fmt.Sprintf("sh -c %q", script))
}

// LimaShell executes a command inside a Lima VM on the remote host.
func (r *Runner) LimaShell(ctx context.Context, vmName, command string) (string, error) {
	return r.Run(ctx, fmt.Sprintf("limactl shell %s -- %s", vmName, command))
}

// LimaShellSudo executes a command as root inside a Lima VM on the remote host.
func (r *Runner) LimaShellSudo(ctx context.Context, vmName, command string) (string, error) {
	return r.Run(ctx, fmt.Sprintf("limactl shell %s -- sudo sh -c %q", vmName, command))
}

// isTransient returns true if the error message suggests a transient SSH failure.
func isTransient(errMsg string) bool {
	transientPatterns := []string{
		"Connection refused",
		"Connection reset",
		"Connection timed out",
		"No route to host",
		"ssh_exchange_identification",
		"kex_exchange_identification",
		"Host is down",
		"Network is unreachable",
	}
	for _, p := range transientPatterns {
		if strings.Contains(errMsg, p) {
			return true
		}
	}
	return false
}
