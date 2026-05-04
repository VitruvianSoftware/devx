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

package ai

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ─── Styled Output ───────────────────────────────────────────────────────────

var (
	diagBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#E3B341")).
			Padding(0, 1).
			MarginTop(1)

	diagHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E3B341"))

	diagBodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E6EDF3"))

	diagSuggestionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#79C0FF")).
				Bold(true)

	diagAILabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8B949E")).
				Italic(true)
)

// ─── Rule Engine ─────────────────────────────────────────────────────────────

// diagnosisRule defines a pattern-matching rule for common failures.
type diagnosisRule struct {
	Pattern    string // Substring to match in error message or stderr
	ExitCode   int    // If non-zero, only match when exit code equals this
	Diagnosis  string // Human-readable explanation
	Suggestion string // Actionable command or fix
}

// rules is the built-in knowledge base of common devx failure patterns.
var rules = []diagnosisRule{
	{
		Pattern:    "password authentication failed",
		Diagnosis:  "Database credentials mismatch. The password your app is using doesn't match the one the database container was started with.",
		Suggestion: "devx config pull   # sync credentials from your vault",
	},
	{
		Pattern:    "address already in use",
		Diagnosis:  "Port conflict — another process is already listening on the required port.",
		Suggestion: "lsof -i :<port>   # find the conflicting process, then kill it",
	},
	{
		Pattern:    "connection refused",
		Diagnosis:  "The target service is not running or not accepting connections yet. This usually means a database or dependency hasn't finished starting.",
		Suggestion: "devx db spawn <engine>   # ensure databases are running",
	},
	{
		Pattern:    "no such container",
		Diagnosis:  "The container was removed or never created. Your local topology may be out of sync.",
		Suggestion: "devx up   # recreate all containers from devx.yaml",
	},
	{
		Pattern:    "ECONNREFUSED 127.0.0.1:11434",
		Diagnosis:  "Ollama is not running. AI features require the Ollama inference server.",
		Suggestion: "ollama serve   # start the local inference server",
	},
	{
		Pattern:    "certificate has expired",
		Diagnosis:  "A TLS certificate has expired — likely the Cloudflare tunnel or cloudflared login session.",
		Suggestion: "devx doctor auth   # refresh expired credentials",
	},
	{
		Pattern:    "context deadline exceeded",
		Diagnosis:  "A healthcheck or connection timed out. The service may be starting too slowly or is unresponsive.",
		Suggestion: "Increase healthcheck.timeout in devx.yaml, or check if the service is crashing during startup",
	},
	{
		Pattern:    "manifest unknown",
		Diagnosis:  "The container image was not found in the registry. The image name or tag may be wrong.",
		Suggestion: "Verify the image name in devx.yaml — check for typos in the tag or registry path",
	},
	{
		Pattern:    "image not found",
		Diagnosis:  "The container image doesn't exist locally or in the remote registry.",
		Suggestion: "Verify the image name in devx.yaml, or pull it manually: podman pull <image>",
	},
	{
		Pattern:    "permission denied",
		Diagnosis:  "Insufficient permissions. This could be a file ownership issue or missing RBAC in Kubernetes.",
		Suggestion: "Check file permissions, or run: devx bridge rbac   # for K8s RBAC issues",
	},
	{
		Pattern:    "gcloud.auth.docker-helper",
		Diagnosis:  "Your Google Cloud credential helper token has expired, blocking container image pulls.",
		Suggestion: "gcloud auth login   # refresh your GCP authentication",
	},
	{
		Pattern:    "OOMKilled",
		Diagnosis:  "The container was killed because it ran out of memory.",
		Suggestion: "devx vm resize --memory 8192   # increase VM memory allocation",
	},
	{
		Pattern:    "exec format error",
		Diagnosis:  "Architecture mismatch — the image was built for a different CPU architecture (e.g., amd64 vs arm64).",
		Suggestion: "Use a multi-arch image, or specify --platform linux/arm64 in your container config",
	},
	{
		Pattern:    "not running",
		Diagnosis:  "The target container or service is not in a running state.",
		Suggestion: "devx up   # start all services defined in devx.yaml",
	},
	{
		Pattern:    "CF_TUNNEL_TOKEN",
		Diagnosis:  "The Cloudflare tunnel token is missing from your environment.",
		Suggestion: "devx doctor auth   # configure tunnel credentials",
	},
	{
		Pattern:    "admin:org",
		Diagnosis:  "Your GitHub CLI session is missing the 'admin:org' scope required for this operation.",
		Suggestion: "gh auth refresh -s admin:org   # add the missing scope",
	},
}

// ─── Public API ──────────────────────────────────────────────────────────────

// DiagnoseFailure attempts to explain a devx command failure using a two-tier
// approach: (1) rule-based pattern matching against known failure modes, then
// (2) AI-enhanced diagnosis if a local LLM or cloud API is available.
//
// Returns a styled, printable string, or "" if no diagnosis is available.
// This function never returns an error — it degrades gracefully.
func DiagnoseFailure(cmdName string, exitCode int, errMsg string, stderr string) string {
	combined := errMsg + "\n" + stderr

	// ── Tier 1: Rule-based pattern matching ──────────────────────────────
	if diagnosis := matchRule(combined, exitCode); diagnosis != "" {
		return diagnosis
	}

	// ── Tier 2: AI-enhanced diagnosis (silent if unavailable) ────────────
	if diagnosis := aiDiagnose(cmdName, exitCode, errMsg, stderr); diagnosis != "" {
		return diagnosis
	}

	return ""
}

// ─── Tier 1: Rule Engine ─────────────────────────────────────────────────────

// matchRule checks the error text against the built-in knowledge base.
func matchRule(text string, exitCode int) string {
	lower := strings.ToLower(text)

	for _, r := range rules {
		if r.ExitCode != 0 && r.ExitCode != exitCode {
			continue
		}
		if strings.Contains(lower, strings.ToLower(r.Pattern)) {
			return renderDiagnosis(r.Diagnosis, r.Suggestion, "rule-based")
		}
	}

	return ""
}

// ─── Tier 2: AI Diagnosis ────────────────────────────────────────────────────

// aiDiagnose collects runtime context and sends it to the best available AI backend.
func aiDiagnose(cmdName string, exitCode int, errMsg string, stderr string) string {
	// Collect runtime context (best-effort, failures are silent)
	context := collectRuntimeContext()

	prompt := fmt.Sprintf(`A devx CLI command failed. Diagnose the root cause and suggest a fix.

Command: devx %s
Exit Code: %d
Error: %s
Stderr: %s

Runtime Context:
%s

Instructions:
1. Identify the most likely root cause based on the error and runtime context.
2. Be specific — mention exact container names, port numbers, or env var names when relevant.
3. Suggest one actionable fix command the developer can run immediately.
4. Keep your response to 2-3 sentences max. No markdown formatting.`, cmdName, exitCode, errMsg, truncate(stderr, 500), context)

	// Use a tight timeout — the developer is already waiting on a failure
	done := make(chan *AgentResult, 1)
	go func() {
		result, _ := tryChatCompletion(prompt)
		done <- result
	}()

	select {
	case result := <-done:
		if result != nil && result.Output != "" {
			return renderDiagnosis(result.Output, "", "ai")
		}
	case <-time.After(15 * time.Second):
		// AI took too long — don't make a bad experience worse
	}

	return ""
}

// collectRuntimeContext gathers container and system state for AI diagnosis.
func collectRuntimeContext() string {
	var parts []string

	// List all devx containers and their state
	if out, err := exec.Command("podman", "ps", "-a",
		"--filter", "name=devx-",
		"--format", "{{.Names}}\t{{.Status}}\t{{.Ports}}").Output(); err == nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed != "" {
			parts = append(parts, "Containers:\n"+trimmed)
		}
	}

	// Check for obvious environment variable issues
	var envHints []string
	for _, key := range []string{
		"DATABASE_URL", "POSTGRES_PASSWORD", "MYSQL_PASSWORD",
		"OPENAI_API_KEY", "CLOUDFLARE_API_TOKEN", "CF_TUNNEL_TOKEN",
	} {
		if v := os.Getenv(key); v != "" {
			envHints = append(envHints, fmt.Sprintf("  %s = (set, %d chars)", key, len(v)))
		} else {
			envHints = append(envHints, fmt.Sprintf("  %s = (not set)", key))
		}
	}
	if len(envHints) > 0 {
		parts = append(parts, "Environment Variables:\n"+strings.Join(envHints, "\n"))
	}

	if len(parts) == 0 {
		return "(no runtime context available)"
	}
	return strings.Join(parts, "\n\n")
}

// ─── Rendering ───────────────────────────────────────────────────────────────

// renderDiagnosis formats the diagnosis into a styled terminal box.
func renderDiagnosis(diagnosis, suggestion, source string) string {
	header := diagHeaderStyle.Render("💡 Diagnosis")

	body := diagBodyStyle.Render(diagnosis)

	var content string
	if suggestion != "" {
		cmd := diagSuggestionStyle.Render("  → " + suggestion)
		content = header + "\n\n" + body + "\n\n" + cmd
	} else {
		content = header + "\n\n" + body
	}

	label := diagAILabelStyle.Render(fmt.Sprintf("  (%s)", source))
	content += "\n" + label

	return diagBoxStyle.Render(content)
}

// truncate limits a string to maxLen characters, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
