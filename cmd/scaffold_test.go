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
	"os"
	"path/filepath"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/testutil"
)

func TestScaffoldCommand(t *testing.T) {
	fake := testutil.SetupFakeRuntime(t)

	// Create a dummy destination directory so the test can safely run
	destDir := t.TempDir()
	targetPath := filepath.Join(destDir, "my-app")

	// Adjust arguments for the scaffold command
	// Notice we can't test error exiting easily if the command calls os.Exit,
	// but devx commands generally return error from RunE.
	NonInteractive = true
	defer func() { NonInteractive = false }()
	err := scaffoldCmd.RunE(scaffoldCmd, []string{"go-api", targetPath})

	// The code currently checks if the command was successful.
	// Since we mock `git`, `docker`, `podman`, they all return 0 exit codes automatically.
	if err != nil {
		t.Fatalf("expected scaffold to succeed, got %v", err)
	}

	requests := fake.Requests(t)
	if len(requests) == 0 {
		t.Fatalf("expected subcommands to be executed, but none were captured")
	}

	// Verify standard git init was attempted via the embedded scaffolding engine
	gitInitFound := false
	for _, req := range requests {
		if req[0] == "git" && len(req) >= 2 && req[1] == "init" {
			gitInitFound = true
		}
	}

	if !gitInitFound {
		t.Errorf("expected to find 'git init' in execution log, got:\n%v", requests)
	}
}

func TestScaffoldForce(t *testing.T) {
	fake := testutil.SetupFakeRuntime(t)

	destDir := t.TempDir()
	targetPath := filepath.Join(destDir, "my-app")

	// Create the directory so that "--force" is required to bypass the idempotency guard
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Make sure the force flag is active
	_ = scaffoldCmd.Flags().Set("force", "true")
	defer func() { _ = scaffoldCmd.Flags().Set("force", "false") }() // reset for other tests

	NonInteractive = true
	defer func() { NonInteractive = false }()
	err := scaffoldCmd.RunE(scaffoldCmd, []string{"go-api", targetPath})
	if err != nil {
		t.Fatalf("expected scaffold with --force to succeed even if dir exists, got: %v", err)
	}

	requests := fake.Requests(t)
	// Just verify something executed signifying it bypassed the guard
	if len(requests) == 0 {
		t.Fatalf("--force bypass failed: expected subcommands but none executed")
	}
}
