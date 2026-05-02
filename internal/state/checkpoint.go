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

package state

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/VitruvianSoftware/devx/internal/provider"
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
func CreateCheckpoint(providerName, name string, rt provider.ContainerRuntime) error {
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

	// Clean up partial checkpoint directory on any failure
	var checkpointErr error
	defer func() {
		if checkpointErr != nil {
			_ = os.RemoveAll(targetDir)
		}
	}()

	// 1. Discover all devx-managed containers that are running
	containers, err := getRunningDevxContainers(rt)
	if err != nil {
		checkpointErr = err
		return err
	}
	if len(containers) == 0 {
		checkpointErr = fmt.Errorf("no running devx-managed containers found to checkpoint")
		return checkpointErr
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
		checkpointErr = fmt.Errorf("checkpoint failed with errors:\n%s", strings.Join(combinedErrs, "\n"))
		return checkpointErr
	}

	return nil
}

// RestoreCheckpoint restores all containers associated with the named checkpoint.
// Restores are performed sequentially to avoid port-binding races when CRIU
// re-binds the original network sockets.
func RestoreCheckpoint(providerName, name string, rt provider.ContainerRuntime) error {
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
	_ = teardownRunningDevxContainers(rt)

	fmt.Printf("🔄 Restoring %d containers from checkpoint %q...\n", len(archives), name)

	// Restore sequentially to avoid port-binding races during CRIU socket restoration
	var restoreErrs []string
	for _, arch := range archives {
		fmt.Printf("  → Restoring %s...\n", filepath.Base(arch))
		cmd := exec.Command("podman", "container", "restore", "-i", arch)
		if out, err := cmd.CombinedOutput(); err != nil {
			restoreErrs = append(restoreErrs, fmt.Sprintf("failed to restore %s: %v\n%s", filepath.Base(arch), err, string(out)))
		}
	}

	if len(restoreErrs) > 0 {
		return fmt.Errorf("restore failed with errors:\n%s", strings.Join(restoreErrs, "\n"))
	}

	return nil
}

// getRunningDevxContainers parses 'podman ps' and returns IDs of devx- prefix containers.
func getRunningDevxContainers(rt provider.ContainerRuntime) ([]string, error) {
	cmdArgs := []string{"ps", "--filter", "label=devx-managed=true", "--format", "{{.Names}}"}
	out, err := rt.Exec(cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
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
func teardownRunningDevxContainers(rt provider.ContainerRuntime) error {
	containers, err := getRunningDevxContainers(rt)
	if err != nil || len(containers) == 0 {
		return nil
	}
	args := append([]string{"rm", "-f"}, containers...)
	_, err = rt.Exec(args...)
	return err
}

// CheckpointInfo holds metadata about a stored checkpoint.
type CheckpointInfo struct {
	Name           string `json:"name"`
	ContainerCount int    `json:"container_count"`
	SizeBytes      int64  `json:"size_bytes"`
	CreatedAt      string `json:"created_at"`
}

// ListCheckpoints returns metadata for all existing checkpoints.
func ListCheckpoints() ([]CheckpointInfo, error) {
	entries, err := os.ReadDir(CheckpointsDir())
	if os.IsNotExist(err) {
		return []CheckpointInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	var results []CheckpointInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cpDir := filepath.Join(CheckpointsDir(), e.Name())

		// Count archives and sum sizes
		archives, _ := filepath.Glob(filepath.Join(cpDir, "*.tar.gz"))
		var totalSize int64
		for _, a := range archives {
			if fi, err := os.Stat(a); err == nil {
				totalSize += fi.Size()
			}
		}

		// Use directory mod time as creation proxy
		info, _ := e.Info()
		created := ""
		if info != nil {
			created = info.ModTime().Format("2006-01-02 15:04:05")
		}

		results = append(results, CheckpointInfo{
			Name:           e.Name(),
			ContainerCount: len(archives),
			SizeBytes:      totalSize,
			CreatedAt:      created,
		})
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
