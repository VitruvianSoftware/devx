package database

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SnapshotMeta holds metadata about a database snapshot.
type SnapshotMeta struct {
	Name      string    `json:"name"`
	Engine    string    `json:"engine"`
	Volume    string    `json:"volume"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
}

const snapshotDirEnv = "DEVX_SNAPSHOT_DIR"

// SnapshotDir returns the directory where snapshots are stored.
func SnapshotDir() string {
	if d := os.Getenv(snapshotDirEnv); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devx", "snapshots")
}

func snapshotPath(engine, name string) string {
	return filepath.Join(SnapshotDir(), engine, name+".tar")
}

func metaPath(engine, name string) string {
	return filepath.Join(SnapshotDir(), engine, name+".json")
}

// CreateSnapshot exports the named volume for the given engine into a tar archive.
// It uses the 'runtime' binary (podman or docker).
func CreateSnapshot(runtime, engine, snapshotName string) (*SnapshotMeta, error) {
	volumeName := fmt.Sprintf("devx-data-%s", engine)

	if err := os.MkdirAll(filepath.Join(SnapshotDir(), engine), 0755); err != nil {
		return nil, fmt.Errorf("could not create snapshot directory: %w", err)
	}

	tarPath := snapshotPath(engine, snapshotName)

	// Prevent overwriting an existing snapshot without being explicit
	if _, err := os.Stat(tarPath); err == nil {
		return nil, fmt.Errorf("snapshot %q already exists — use a different name or delete it first with: devx db snapshot rm %s %s", snapshotName, engine, snapshotName)
	}

	fmt.Printf("📸 Snapshotting volume %s → %s...\n", volumeName, tarPath)

	// podman volume export <volume> --output <path>
	// docker doesn't have 'volume export', so we use a helper container for docker
	var cmd *exec.Cmd
	if runtime == "podman" {
		cmd = exec.Command("podman", "volume", "export", volumeName, "--output", tarPath)
	} else {
		// For docker: spin up a lightweight helper container that tars the volume contents
		cmd = exec.Command("docker", "run", "--rm",
			"-v", volumeName+":/data:ro",
			"-v", filepath.Dir(tarPath)+":/out",
			"alpine", "tar", "-czf", "/out/"+filepath.Base(tarPath), "-C", "/data", ".")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Clean up partial output
		_ = os.Remove(tarPath)
		return nil, fmt.Errorf("snapshot failed: %w", err)
	}

	fi, err := os.Stat(tarPath)
	if err != nil {
		return nil, fmt.Errorf("snapshot file missing after export: %w", err)
	}

	meta := &SnapshotMeta{
		Name:      snapshotName,
		Engine:    engine,
		Volume:    volumeName,
		CreatedAt: time.Now(),
		SizeBytes: fi.Size(),
	}

	mb, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("could not write snapshot metadata: %w", err)
	}
	if err := os.WriteFile(metaPath(engine, snapshotName), mb, 0644); err != nil {
		return nil, fmt.Errorf("could not persist snapshot metadata: %w", err)
	}

	return meta, nil
}

// RestoreSnapshot stops the running container, replaces the volume with the
// snapshot contents, and restarts the container.
func RestoreSnapshot(runtime, engine, snapshotName string) error {
	volumeName := fmt.Sprintf("devx-data-%s", engine)
	containerName := fmt.Sprintf("devx-db-%s", engine)
	tarPath := snapshotPath(engine, snapshotName)

	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		return fmt.Errorf("snapshot %q not found — run 'devx db snapshot list %s' to see available snapshots", snapshotName, engine)
	}

	fmt.Printf("🔄 Restoring snapshot %q into volume %s...\n", snapshotName, volumeName)

	// Step 1: Stop the running database container (non-fatal if not running)
	fmt.Println("  → Stopping container...")
	_ = exec.Command(runtime, "stop", containerName).Run()

	// Step 2: Remove the existing volume so we can restore cleanly
	fmt.Println("  → Removing existing volume data...")
	if err := exec.Command(runtime, "volume", "rm", "-f", volumeName).Run(); err != nil {
		return fmt.Errorf("could not remove existing volume: %w", err)
	}

	// Step 3: Re-create the volume
	if err := exec.Command(runtime, "volume", "create", volumeName).Run(); err != nil {
		return fmt.Errorf("could not recreate volume: %w", err)
	}

	// Step 4: Import the snapshot
	fmt.Printf("  → Importing %s...\n", tarPath)
	var importCmd *exec.Cmd
	if runtime == "podman" {
		importCmd = exec.Command("podman", "volume", "import", volumeName, tarPath)
	} else {
		// Docker: use helper container to expand tar into the volume
		importCmd = exec.Command("docker", "run", "--rm",
			"-v", volumeName+":/data",
			"-v", filepath.Dir(tarPath)+":/src:ro",
			"alpine", "tar", "-xzf", "/src/"+filepath.Base(tarPath), "-C", "/data")
	}

	importCmd.Stdout = os.Stdout
	importCmd.Stderr = os.Stderr
	if err := importCmd.Run(); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	// Step 5: Restart the container
	fmt.Printf("  → Restarting container %s...\n", containerName)
	_ = exec.Command(runtime, "start", containerName).Run()

	fmt.Printf("\n✅ Snapshot %q restored into %s\n", snapshotName, volumeName)
	return nil
}

// ListSnapshots returns all snapshots for a given engine.
func ListSnapshots(engine string) ([]SnapshotMeta, error) {
	dir := filepath.Join(SnapshotDir(), engine)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []SnapshotMeta{}, nil
	}
	if err != nil {
		return nil, err
	}

	var snapshots []SnapshotMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var m SnapshotMeta
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		snapshots = append(snapshots, m)
	}
	return snapshots, nil
}

// DeleteSnapshot removes a snapshot's tar and metadata files.
func DeleteSnapshot(engine, snapshotName string) error {
	tar := snapshotPath(engine, snapshotName)
	meta := metaPath(engine, snapshotName)

	if _, err := os.Stat(tar); os.IsNotExist(err) {
		return fmt.Errorf("snapshot %q not found", snapshotName)
	}

	if err := os.Remove(tar); err != nil {
		return fmt.Errorf("could not delete snapshot archive: %w", err)
	}
	_ = os.Remove(meta) // best-effort
	return nil
}
