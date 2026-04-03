package state

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// CheckpointsDir returns the directory where state checkpoints are stored.
func CheckpointsDir() string {
	if d := os.Getenv("DEVX_CHECKPOINT_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devx", "checkpoints")
}

func checkpointPath(name string) string {
	return filepath.Join(CheckpointsDir(), name)
}

// CreateCheckpoint snapshots all running devx-managed containers using podman's CRIU integration.
func CreateCheckpoint(providerName, name string) error {
	if providerName != "podman" {
		return fmt.Errorf("state checkpoints (CRIU) are only supported on the native podman provider. Current provider: %s", providerName)
	}

	targetDir := checkpointPath(name)
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("checkpoint %q already exists at %s", name, targetDir)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	// 1. Discover all devx-managed containers that are running
	containers, err := getRunningDevxContainers()
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return fmt.Errorf("no running devx-managed containers found to checkpoint")
	}

	fmt.Printf("📸 Checkpointing %d containers into %q...\n", len(containers), targetDir)

	var wg sync.WaitGroup
	errs := make(chan error, len(containers))

	for _, id := range containers {
		wg.Add(1)
		go func(containerId string) {
			defer wg.Done()
			archivePath := filepath.Join(targetDir, containerId+".tar.gz")
			
			// Command: podman container checkpoint <container> --export <archive> --keep
			// Using --keep so we can resume the original container or at least retain volumes properly
			cmd := exec.Command("podman", "container", "checkpoint", containerId, "-e", archivePath, "--keep")
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("failed to checkpoint %s: %w\n%s", containerId, err, string(out))
			}
		}(id)
	}

	wg.Wait()
	close(errs)

	var combinedErrs []string
	for e := range errs {
		combinedErrs = append(combinedErrs, e.Error())
	}

	if len(combinedErrs) > 0 {
		return fmt.Errorf("checkpoint failed with errors:\n%s", strings.Join(combinedErrs, "\n"))
	}

	return nil
}

// RestoreCheckpoint restores all containers associated with the named checkpoint.
func RestoreCheckpoint(providerName, name string) error {
	if providerName != "podman" {
		return fmt.Errorf("state restores (CRIU) are only supported on the native podman provider. Current provider: %s", providerName)
	}

	targetDir := checkpointPath(name)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint %q not found", name)
	}

	archives, err := filepath.Glob(filepath.Join(targetDir, "*.tar.gz"))
	if err != nil || len(archives) == 0 {
		return fmt.Errorf("no checkpoint archives found in %s", targetDir)
	}

	// Because restoring re-binds ports, we must make sure the containers aren't already running.
	// For devx containers, we kill them if they are running.
	_ = teardownRunningDevxContainers()

	fmt.Printf("🔄 Restoring %d containers from checkpoint %q...\n", len(archives), name)

	var wg sync.WaitGroup
	errs := make(chan error, len(archives))

	for _, arch := range archives {
		wg.Add(1)
		go func(archivePath string) {
			defer wg.Done()
			
			// Command: podman container restore --import <archive>
			cmd := exec.Command("podman", "container", "restore", "-i", archivePath)
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("failed to restore %s: %w\n%s", filepath.Base(archivePath), err, string(out))
			}
		}(arch)
	}

	wg.Wait()
	close(errs)

	var combinedErrs []string
	for e := range errs {
		combinedErrs = append(combinedErrs, e.Error())
	}

	if len(combinedErrs) > 0 {
		return fmt.Errorf("restore failed with errors:\n%s", strings.Join(combinedErrs, "\n"))
	}

	return nil
}

// getRunningDevxContainers parses 'podman ps' and returns IDs of devx- prefix containers.
func getRunningDevxContainers() ([]string, error) {
	out, err := exec.Command("podman", "ps", "--format", "{{.Names}}").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("podman ps failed: %w", err)
	}

	var results []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "devx-") {
			results = append(results, line)
		}
	}
	return results, nil
}

// teardownRunningDevxContainers force stops current devx- containers to prevent port collisions on restore
func teardownRunningDevxContainers() error {
	containers, _ := getRunningDevxContainers()
	if len(containers) > 0 {
		args := append([]string{"rm", "-f"}, containers...)
		_ = exec.Command("podman", args...).Run()
	}
	return nil
}

// ListCheckpoints returns a list of all existing checkpoint names.
func ListCheckpoints() ([]string, error) {
	entries, err := os.ReadDir(CheckpointsDir())
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var results []string
	for _, e := range entries {
		if e.IsDir() {
			results = append(results, e.Name())
		}
	}
	return results, nil
}

// DeleteCheckpoint completely removes a checkpoint and its archives.
func DeleteCheckpoint(name string) error {
	targetDir := checkpointPath(name)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint %q not found", name)
	}
	return os.RemoveAll(targetDir)
}
