package cmd

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VitruvianSoftware/devx/internal/testutil"
)

func TestShellCommand(t *testing.T) {
	fake := testutil.SetupFakeRuntime(t)

	testDir := t.TempDir()

	// 1. Create a dummy devcontainer.json
	devcontainerJSON := `{
		"name": "Test Env",
		"image": "ubuntu:latest",
		"workspaceFolder": "/app",
		"containerEnv": {
			"STATIC_ENV": "static_value"
		}
	}`
	err := os.WriteFile(filepath.Join(testDir, ".devcontainer.json"), []byte(devcontainerJSON), 0644)
	if err != nil {
		t.Fatalf("failed to write devcontainer.json: %v", err)
	}

	// 2. Create a dummy .env
	envFile := `
SECRET_API_KEY=supersecret
`
	err = os.WriteFile(filepath.Join(testDir, ".env"), []byte(envFile), 0644)
	if err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// 3. Mock Ollama running locally by opening port 11434 (ignore failure if already running)
	l, err := net.Listen("tcp", "127.0.0.1:11434")
	if err == nil {
		defer l.Close()
	}

	// Give the listener a tiny bit of time to bind
	time.Sleep(10 * time.Millisecond)

	// Change working directory to the test dir
	originalWd, _ := os.Getwd()
	_ = os.Chdir(testDir)
	defer func() { _ = os.Chdir(originalWd) }()

	shellProviderFlag = "podman"

	// Execute shell command
	err = shellCmd.RunE(shellCmd, []string{})
	if err != nil {
		t.Fatalf("expected shell to succeed, got %v", err)
	}

	requests := fake.Requests(t)
	if len(requests) == 0 {
		t.Fatalf("expected subcommands to be executed, but none were captured")
	}

	// Find the container run command
	var runCmd []string
	for _, req := range requests {
		if req[0] == "podman" && len(req) >= 2 && req[1] == "run" {
			runCmd = req
			break
		}
	}

	if runCmd == nil {
		t.Fatalf("expected to find 'podman run' in execution log, got:\n%v", requests)
	}

	runCmdStr := strings.Join(runCmd, " ")

	// Verify devcontainer.json parsing
	if !strings.Contains(runCmdStr, "ubuntu:latest") {
		t.Errorf("expected podman run to use image ubuntu:latest, got: %s", runCmdStr)
	}
	if !strings.Contains(runCmdStr, "-w /app") {
		t.Errorf("expected podman run to set workspace folder, got: %s", runCmdStr)
	}
	if !strings.Contains(runCmdStr, "-e STATIC_ENV=static_value") {
		t.Errorf("expected podman run to inject STATIC_ENV from devcontainer.json, got: %s", runCmdStr)
	}

	// Verify .env injection via fallback envvault
	if !strings.Contains(runCmdStr, "-e SECRET_API_KEY=supersecret") {
		t.Errorf("expected podman run to inject SECRET_API_KEY from .env, got: %s", runCmdStr)
	}

	// Verify AI bridge injection logic (since Ollama is mocked)
	expectedAIVars := []string{
		"-e OPENAI_API_BASE=http://host.containers.internal:11434/v1",
		"-e OPENAI_API_KEY=devx-local-ai",
	}
	for _, expected := range expectedAIVars {
		if !strings.Contains(runCmdStr, expected) {
			t.Errorf("expected podman run to inject AI bridge var: %s, got: %s", expected, runCmdStr)
		}
	}
}
