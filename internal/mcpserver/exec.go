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

// Package mcpserver exposes devx subcommands as a Model Context Protocol
// server over stdio. Each tool shells out to the devx binary itself with the
// appropriate flags, captures stdout/stderr, and returns the result to the
// agent host. Subprocess isolation keeps the MCP server's own stdout (which
// is the JSON-RPC transport) free of cobra-generated chatter.
package mcpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// devxBinary is overridable for tests. In production it points to the
// currently-running binary so the MCP server always invokes the same devx it
// was launched from.
var devxBinary = func() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return "devx"
}()

// runDevx executes the devx binary with the given args and returns stdout. A
// non-zero exit code is wrapped together with stderr so the caller can surface
// both the structured exit code (devx codes 15-99) and any diagnostic output.
func runDevx(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, devxBinary, args...)
	cmd.Env = append(os.Environ(),
		// Force non-interactive paths even for commands that didn't get a -y.
		"DEVX_NON_INTERACTIVE=1",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return stdout.String(), fmt.Errorf("devx %v failed (exit %d): %s",
				args, exitErr.ExitCode(), trimForError(stderr.String()))
		}
		return stdout.String(), fmt.Errorf("devx %v failed: %w", args, err)
	}
	return stdout.String(), nil
}

func trimForError(s string) string {
	const max = 2048
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
