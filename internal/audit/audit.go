// Package audit provides pre-push vulnerability and secret scanning.
// It uses Trivy for CVE/vulnerability scanning and Gitleaks for secret detection.
// Tools are run natively if available on the host; otherwise they fall back to
// an ephemeral container mount — no user installation required.
package audit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ToolMode describes how an audit tool should be executed.
type ToolMode int

const (
	ModeNative    ToolMode = iota // Run the binary directly from $PATH
	ModeContainer                 // Run via podman/docker with cwd mounted read-only
)

// Tool represents a single audit tool.
type Tool struct {
	Name            string
	BinaryName      string                            // binary to look for in $PATH
	Image           string                            // fallback container image
	NetworkIsolated bool                              // true = run container with --network none
	BuildArgs       func(cwd, format string) []string // args for native execution
	ContainerArgs   func(cwd, format string) []string // args for container execution
}

// Trivy scans for CVEs in OS packages and language dependencies.
// NetworkIsolated = false: Trivy needs internet access to download/update its CVE database.
var Trivy = Tool{
	Name:            "Trivy",
	BinaryName:      "trivy",
	Image:           "ghcr.io/aquasecurity/trivy:latest",
	NetworkIsolated: false,
	BuildArgs: func(cwd, format string) []string {
		args := []string{"fs", "--exit-code", "1", "--no-progress"}
		if format == "json" {
			args = append(args, "--format", "json")
		} else {
			args = append(args, "--format", "table")
		}
		// Respect .trivyignore if present
		ignorePath := cwd + "/.trivyignore"
		if _, err := os.Stat(ignorePath); err == nil {
			args = append(args, "--ignorefile", ignorePath)
		}
		args = append(args, cwd)
		return args
	},
	ContainerArgs: func(cwd, format string) []string {
		// In container mode, cwd is mounted at /scan
		args := []string{"fs", "--exit-code", "1", "--no-progress"}
		if format == "json" {
			args = append(args, "--format", "json")
		} else {
			args = append(args, "--format", "table")
		}
		// Respect .trivyignore mounted inside the container at /scan/.trivyignore
		ignorePath := cwd + "/.trivyignore"
		if _, err := os.Stat(ignorePath); err == nil {
			args = append(args, "--ignorefile", "/scan/.trivyignore")
		}
		args = append(args, "/scan")
		return args
	},
}

// Gitleaks scans git history and working tree for leaked secrets.
// NetworkIsolated = true: Gitleaks needs no network access — pure local filesystem scan.
var Gitleaks = Tool{
	Name:            "Gitleaks",
	BinaryName:      "gitleaks",
	Image:           "docker.io/zricethezav/gitleaks:latest",
	NetworkIsolated: true,
	BuildArgs: func(cwd, format string) []string {
		args := []string{"detect", "--source", cwd, "--no-git", "--exit-code", "1"}
		if format == "json" {
			args = append(args, "--report-format", "json", "--report-path", "/dev/stdout")
		}
		return args
	},
	ContainerArgs: func(cwd, format string) []string {
		args := []string{"detect", "--source", "/scan", "--no-git", "--exit-code", "1"}
		if format == "json" {
			args = append(args, "--report-format", "json", "--report-path", "/dev/stdout")
		}
		return args
	},
}

// Detect returns the execution mode for a tool — native if the binary is in
// $PATH, container otherwise. It also returns which runtime to use.
func Detect(t Tool) (ToolMode, string) {
	if _, err := exec.LookPath(t.BinaryName); err == nil {
		return ModeNative, ""
	}
	// Prefer podman, fall back to docker
	if _, err := exec.LookPath("podman"); err == nil {
		return ModeContainer, "podman"
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return ModeContainer, "docker"
	}
	return ModeContainer, ""
}

// ErrVMNotRunning is returned when the container runtime is present but the
// underlying VM (podman machine) is not started.
var ErrVMNotRunning = fmt.Errorf("podman VM is not running")

// Run executes the tool against the given directory, returning combined output
// and any error. exitCode 1 from the scanner is treated as "found issues".
func Run(t Tool, cwd, runtime, format string) (string, bool, error) {
	mode, rt := Detect(t)
	if runtime != "" {
		rt = runtime
	}
	if mode == ModeContainer && rt == "" {
		return "", false, fmt.Errorf(
			"%s is not installed and no container runtime was found. Install %s or podman/docker",
			t.Name, t.BinaryName,
		)
	}

	var cmd *exec.Cmd
	if mode == ModeNative {
		args := t.BuildArgs(cwd, format)
		cmd = exec.Command(t.BinaryName, args...)
	} else {
		// ── Pre-check: verify the container daemon is actually reachable ──────
		// This catches the "podman machine not started" case before we attempt
		// a pull and get a wall of confusing daemon error text.
		if err := checkContainerRuntime(rt); err != nil {
			return "", false, err
		}

		// Pull the image, bypassing the gcloud Docker credential helper.
		// gcloud-auth-docker intercepts ALL registries (including public docker.io),
		// causing auth failures for images that need no credentials at all.
		// We write a temp file containing valid empty JSON {} and point
		// REGISTRY_AUTH_FILE at it. Podman parses it as "no credentials configured"
		// and falls back to anonymous auth — which is exactly what we need.
		// NOTE: we do NOT set this on `podman run` because by then the image is
		// already pulled and cached locally; no auth is required.
		authFile, authErr := os.CreateTemp("", "devx-audit-auth-*.json")
		if authErr == nil {
			_, _ = authFile.WriteString("{}")
			authFile.Close()
			defer os.Remove(authFile.Name())
		}

		pullCmd := exec.Command(rt, "pull", "--quiet", t.Image)
		if authErr == nil {
			pullCmd.Env = append(os.Environ(), "REGISTRY_AUTH_FILE="+authFile.Name())
		}
		_ = pullCmd.Run()

		networkArg := "bridge" // default: allow internet (e.g. Trivy DB download)
		if t.NetworkIsolated {
			networkArg = "none" // Gitleaks: pure filesystem scan, no network needed
		}
		containerArgs := append([]string{
			"run", "--rm", "--network", networkArg,
			"-v", cwd + ":/scan:ro",
		}, t.Image)
		containerArgs = append(containerArgs, t.ContainerArgs(cwd, format)...)
		cmd = exec.Command(rt, containerArgs...)
		// Apply the same bypass on run: if the pull failed or was skipped, the
		// image may still need to be fetched at run time. {} is a valid empty
		// auth config that tells Podman to use anonymous auth for all registries.
		if authErr == nil {
			cmd.Env = append(os.Environ(), "REGISTRY_AUTH_FILE="+authFile.Name())
		}
	}

	cmd.Stdin = os.Stdin
	out, err := cmd.CombinedOutput()

	foundIssues := false
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Exit code 1 = issues found (not a tool crash) — not an error condition
			foundIssues = true
		} else {
			// Surface the raw output so the user can see exactly what went wrong
			return string(out), false, fmt.Errorf("%s failed: %w\n%s", t.Name, err, string(out))
		}
	}
	return string(out), foundIssues, nil
}

// checkContainerRuntime pings the runtime daemon and returns a descriptive
// error if it's not reachable (e.g. podman machine is sleeping).
func checkContainerRuntime(rt string) error {
	out, err := exec.Command(rt, "info", "--format", "{{.Host.OS}}").CombinedOutput()
	if err == nil {
		return nil // daemon is up
	}
	// Detect the "VM not started" case specifically
	if strings.Contains(string(out), "Cannot connect to Podman") ||
		strings.Contains(string(out), "connection refused") ||
		strings.Contains(string(out), "no such file or directory") {
		return ErrVMNotRunning
	}
	return fmt.Errorf("container runtime %q is not reachable: %w", rt, err)
}

// InstallPrePushHook writes a git pre-push hook to .git/hooks/pre-push.
func InstallPrePushHook(cwd string) error {
	hooksDir := cwd + "/.git/hooks"
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository (no .git/hooks directory found)")
	}
	hookPath := hooksDir + "/pre-push"

	// Don't overwrite an existing hook without reading it
	if _, err := os.Stat(hookPath); err == nil {
		existing, _ := os.ReadFile(hookPath)
		if strings.Contains(string(existing), "devx audit") {
			return nil // already installed
		}
		return fmt.Errorf("a pre-push hook already exists at %s — add 'devx audit' manually", hookPath)
	}

	hook := `#!/bin/sh
# Installed by devx audit install-hooks
# Runs secret and vulnerability scanning before every push.
set -e
echo "🔍 devx audit: scanning for secrets and vulnerabilities..."
devx audit
`
	if err := os.WriteFile(hookPath, []byte(hook), 0755); err != nil {
		return fmt.Errorf("failed to write hook: %w", err)
	}
	return nil
}
