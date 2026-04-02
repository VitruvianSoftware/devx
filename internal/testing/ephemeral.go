// Package testing provides the ephemeral test topology engine for devx test ui.
// It provisions short-lived, isolated database containers on random ports so that
// E2E/Playwright/Cypress tests can run in a clean environment without touching
// the developer's active development databases.
//
// # One-to-one engine mapping
//
// devx db spawn only supports ONE container per engine type (e.g. a single
// devx-db-postgres). The ephemeral engine mirrors this 1:1 so each engine in
// devx.yaml gets exactly one ephemeral clone. Supporting multiple independent
// instances of the same engine type (e.g., two separate Postgres databases) is
// intentionally deferred to a future iteration when real use-cases emerge.
package testing

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/database"
)

// EphemeralDB holds metadata about a single ephemeral database container.
type EphemeralDB struct {
	Engine        database.Engine
	EngineName    string
	ContainerName string
	VolumeName    string
	HostPort      int
}

// RunFlow is the primary orchestrator for `devx test ui`. It:
//  1. Provisions ephemeral database containers for every engine in dbEngines.
//  2. Runs optional pre-processing steps (setup) with the injected environment.
//  3. Executes the main test command with the injected environment
//  4. Tears down all ephemeral containers and volumes unconditionally when done.
func RunFlow(dbEngines []string, runtime, setup, command string) error {
	if command == "" {
		return fmt.Errorf("no test command specified (use --command or set test.ui.command in devx.yaml)")
	}

	uuid := shortUUID()
	fmt.Printf("🧪 devx test ui — ephemeral run ID: %s\n\n", uuid)

	var dbs []EphemeralDB

	// Provision all ephemeral databases
	for _, engineName := range dbEngines {
		engine, ok := database.Registry[engineName]
		if !ok {
			return fmt.Errorf("unknown database engine %q in devx.yaml", engineName)
		}

		port, err := freePort()
		if err != nil {
			return fmt.Errorf("could not acquire free port for ephemeral %s: %w", engineName, err)
		}

		containerName := fmt.Sprintf("devx-db-%s-ephemeral-%s", engineName, uuid)
		volumeName := fmt.Sprintf("devx-ephemeral-%s-%s", engineName, uuid)

		fmt.Printf("🚀 Booting ephemeral %s on port %d (container: %s)...\n", engine.Name, port, containerName)

		runArgs := []string{
			"run", "-d",
			"--name", containerName,
			"-p", fmt.Sprintf("%d:%d", port, engine.InternalPort),
			"-v", fmt.Sprintf("%s:%s", volumeName, engine.VolumePath),
			"--label", "managed-by=devx",
			"--label", fmt.Sprintf("devx-engine=%s", engineName),
			"--label", "devx-ephemeral=true",
		}
		for k, v := range engine.Env {
			runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", k, v))
		}
		runArgs = append(runArgs, engine.Image)

		spawnCmd := exec.Command(runtime, runArgs...)
		spawnCmd.Stdout = os.Stdout
		spawnCmd.Stderr = os.Stderr
		if err := spawnCmd.Run(); err != nil {
			return fmt.Errorf("failed to spawn ephemeral %s: %w", engineName, err)
		}

		dbs = append(dbs, EphemeralDB{
			Engine:        engine,
			EngineName:    engineName,
			ContainerName: containerName,
			VolumeName:    volumeName,
			HostPort:      port,
		})
	}

	// Unconditional teardown — always runs, even if tests fail or panic
	defer func() {
		fmt.Printf("\n🧹 Tearing down %d ephemeral database(s)...\n", len(dbs))
		for _, db := range dbs {
			fmt.Printf("  ↳ Removing %s...\n", db.ContainerName)
			_ = exec.Command(runtime, "rm", "-f", db.ContainerName).Run()
			_ = exec.Command(runtime, "volume", "rm", "-f", db.VolumeName).Run()
		}
		fmt.Println("✅ Ephemeral topology destroyed. Your development database is untouched.")
	}()

	// Wait briefly for containers to become ready (simple log-polling approach)
	if len(dbs) > 0 {
		fmt.Printf("\n⏳ Waiting for databases to become ready...\n")
		for _, db := range dbs {
			if err := waitForReady(runtime, db, 30*time.Second); err != nil {
				return err
			}
			fmt.Printf("  ✓ %s is ready\n", db.Engine.Name)
		}
	}

	// Build the injected environment
	testEnv := buildEnv(dbs)

	// Run pre-processing setup (e.g., migrations) if specified
	if setup != "" {
		fmt.Printf("\n🔧 Running setup: %s\n", setup)
		if err := runShellStep(setup, testEnv); err != nil {
			return fmt.Errorf("setup step failed: %w", err)
		}
		fmt.Println("  ✓ Setup complete")
	}

	// Run the actual test command
	fmt.Printf("\n🎭 Running tests: %s\n\n", command)
	return runShellStep(command, testEnv)
}

// buildEnv constructs the environment variable slice to inject into child processes.
// For each database it exports DATABASE_URL (the first/primary DB) and
// <ENGINE>_URL for all databases individually.
func buildEnv(dbs []EphemeralDB) []string {
	env := os.Environ()
	primary := true
	for _, db := range dbs {
		connStr := db.Engine.ConnString(db.HostPort)
		// Always export the engine-specific URL
		key := strings.ToUpper(db.EngineName) + "_URL"
		env = append(env, fmt.Sprintf("%s=%s", key, connStr))
		// Export DATABASE_URL for the first (primary) database
		if primary {
			env = append(env, fmt.Sprintf("DATABASE_URL=%s", connStr))
			primary = false
		}
	}
	return env
}

// runShellStep executes a shell command string with the injected environment.
func runShellStep(command string, env []string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// waitForReady polls the container logs for the engine's ReadyLog marker.
func waitForReady(runtime string, db EphemeralDB, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := exec.Command(runtime, "logs", db.ContainerName).CombinedOutput()
		if err == nil && strings.Contains(string(out), db.Engine.ReadyLog) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s to become ready — check: %s logs %s",
		db.Engine.Name, runtime, db.ContainerName)
}

// freePort asks the OS for an available port by binding to :0.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// shortUUID returns a 6-character hex string for container namespacing.
func shortUUID() string {
	// Use nanosecond timestamp XOR'd with pid for uniqueness without crypto/rand overhead
	t := time.Now().UnixNano()
	pid := os.Getpid()
	return fmt.Sprintf("%06x", (t^int64(pid))&0xffffff)
}
