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
