package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/secrets"
)

type FakeRuntime struct {
	BinDir       string
	CallsLogPath string
}

// SetupFakeRuntime provisions an isolated PATH environment injected with shell stubs
// that intercept external tool executions (podman, docker, cloudflared).
func SetupFakeRuntime(t *testing.T) *FakeRuntime {
	t.Helper()

	// Disable interactive prompts globally during test execution to prevent blocking or ANSI garbage
	secrets.NonInteractive = true

	baseDir := t.TempDir()
	binDir := filepath.Join(baseDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("failed to create fake bin dir: %v", err)
	}

	callsLog := filepath.Join(baseDir, "calls.jsonl")

	// The bash stub is extremely simple. It captures the command name ($0) and arguments ($@),
	// serializes them into a JSON array, and appends it to out calls log.
	stubPython := fmt.Sprintf(`#!/usr/bin/env python3
import sys, json, os

with open("%s", "a") as f:
    f.write(json.dumps([os.path.basename(sys.argv[0])] + sys.argv[1:]) + "\n")
`, callsLog)

	stubPath := filepath.Join(binDir, "stub.py")
	if err := os.WriteFile(stubPath, []byte(stubPython), 0755); err != nil {
		t.Fatalf("failed to write stub: %v", err)
	}

	// Create symlinks from the tools we want to intercept, pointing to our stub.
	tools := []string{"podman", "docker", "cloudflared", "git", "gh"}
	for _, tool := range tools {
		linkPath := filepath.Join(binDir, tool)
		if err := os.Symlink(stubPath, linkPath); err != nil {
			t.Fatalf("failed to link %s: %v", tool, err)
		}
	}

	// Setup the PATH
	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+originalPath)

	return &FakeRuntime{
		BinDir:       binDir,
		CallsLogPath: callsLog,
	}
}

// Requests parsed the intercepted invocation log.
// Returns a 2D slice where each sub-slice is [command, arg1, arg2...]
func (f *FakeRuntime) Requests(t *testing.T) [][]string {
	t.Helper()

	content, err := os.ReadFile(f.CallsLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no calls were made
		}
		t.Fatalf("failed to read calls log: %v", err)
	}

	var requests [][]string
	lines := splitLines(string(content))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatalf("failed to decode call log: %v", err)
		}
		requests = append(requests, args)
	}
	return requests
}

func splitLines(s string) []string {
	var lines []string
	curr := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, curr)
			curr = ""
		} else {
			curr += string(r)
		}
	}
	if curr != "" {
		lines = append(lines, curr)
	}
	return lines
}
