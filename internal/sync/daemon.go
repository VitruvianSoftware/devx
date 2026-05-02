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

// Package sync provides a thin Go wrapper around the Mutagen CLI
// for creating, listing, and terminating file sync sessions.
package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Session represents an active Mutagen sync session managed by devx.
type Session struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Dest      string `json:"dest"`
	Container string `json:"container"`
	Status    string `json:"status"`
}

// DefaultIgnores returns the set of patterns that every devx sync session
// excludes by default. These prevent catastrophic performance degradation
// from syncing millions of transient build/runtime files.
func DefaultIgnores() []string {
	return []string{
		".git",
		"node_modules",
		".devx",
		"__pycache__",
		".next",
		".nuxt",
		"dist",
		"build",
	}
}

// EnsureDaemon starts the Mutagen daemon if it is not already running.
func EnsureDaemon() error {
	// mutagen daemon start is idempotent — safe to call even if already running
	cmd := exec.Command("mutagen", "daemon", "start")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// CreateSession creates a Mutagen sync session between a host path and a
// container destination. The session is labeled managed-by=devx and named
// devx-sync-<name> for idempotent management.
//
// If a session with the same name already exists, it is terminated first.
func CreateSession(name, src, dest, container, containerRuntime string, extraIgnores []string) error {
	sessionName := "devx-sync-" + name

	// Terminate existing session if present (idempotent)
	_ = TerminateSession(name)

	// Build ignore flags
	allIgnores := append(DefaultIgnores(), extraIgnores...)

	args := []string{
		"sync", "create",
		"--name", sessionName,
		"--label", "managed-by=devx",
	}
	for _, pattern := range allIgnores {
		args = append(args, "--ignore", pattern)
	}

	// Endpoint: docker://<container>/<dest>
	endpoint := fmt.Sprintf("docker://%s%s", container, dest)
	args = append(args, src, endpoint)

	cmd := exec.Command("mutagen", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// When runtime is podman, inject DOCKER_HOST to point at Podman's
	// Docker-compatible socket so Mutagen's docker:// transport works.
	cmd.Env = os.Environ()
	if containerRuntime == "podman" {
		sock := podmanSocket()
		if sock != "" {
			cmd.Env = append(cmd.Env, "DOCKER_HOST=unix://"+sock)
		}
	}

	return cmd.Run()
}

// TerminateSession terminates a devx-managed Mutagen sync session by name.
func TerminateSession(name string) error {
	sessionName := "devx-sync-" + name
	cmd := exec.Command("mutagen", "sync", "terminate", sessionName)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// TerminateAll terminates all devx-managed Mutagen sync sessions.
func TerminateAll() error {
	cmd := exec.Command("mutagen", "sync", "terminate", "--label-selector", "managed-by=devx")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// ListSessions returns all active devx-managed Mutagen sync sessions.
func ListSessions() ([]Session, error) {
	out, err := exec.Command("mutagen", "sync", "list", "--label-selector", "managed-by=devx").CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		// "no sessions found" is not an error
		if strings.Contains(outStr, "no sessions") || strings.Contains(outStr, "Error: unable to locate requested sessions") {
			return nil, nil
		}
		return nil, fmt.Errorf("mutagen sync list failed: %w (%s)", err, outStr)
	}

	return parseListOutput(string(out)), nil
}

// parseListOutput parses the human-readable mutagen sync list output.
// Each session block starts with "Name: ..." and contains Alpha/Beta URLs and Status.
func parseListOutput(raw string) []Session {
	var sessions []Session
	var current *Session

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Name: ") {
			if current != nil {
				sessions = append(sessions, *current)
			}
			current = &Session{Name: strings.TrimPrefix(line, "Name: ")}
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "Alpha:") {
			current.Source = strings.TrimSpace(strings.TrimPrefix(line, "Alpha:"))
		}
		if strings.HasPrefix(line, "Beta:") {
			beta := strings.TrimSpace(strings.TrimPrefix(line, "Beta:"))
			current.Dest = beta
			// Extract container name from docker://container/path
			if strings.HasPrefix(beta, "docker://") {
				parts := strings.SplitN(strings.TrimPrefix(beta, "docker://"), "/", 2)
				if len(parts) > 0 {
					current.Container = parts[0]
				}
			}
		}
		if strings.HasPrefix(line, "Status:") {
			current.Status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		}
	}
	if current != nil {
		sessions = append(sessions, *current)
	}
	return sessions
}

// podmanSocket returns the path to the Podman Docker-compatible socket.
func podmanSocket() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	candidates := []string{
		// macOS podman machine socket
		filepath.Join(home, ".local/share/containers/podman/machine/podman.sock"),
		// Linux rootless
		fmt.Sprintf("/run/user/%d/podman/podman.sock", os.Getuid()),
	}

	// On macOS, also try the machine-specific path
	if runtime.GOOS == "darwin" {
		candidates = append([]string{
			filepath.Join(home, ".local/share/containers/podman/machine/qemu/podman.sock"),
			filepath.Join(home, ".local/share/containers/podman/machine/podman-machine-default/podman.sock"),
		}, candidates...)
	}

	for _, sock := range candidates {
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}
	return ""
}

// IsInstalled returns true if the mutagen binary is available on PATH.
func IsInstalled() bool {
	_, err := exec.LookPath("mutagen")
	return err == nil
}
