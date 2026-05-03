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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/database"
)

func TestBundleManifestRoundTrip(t *testing.T) {
	original := BundleManifest{
		ID:             "test-bundle-id",
		Mode:           "full",
		CheckpointName: "_share_1234567890",
		Containers:     []string{"web.tar.gz", "api.tar.gz"},
		Databases: []database.SnapshotMeta{
			{Engine: "postgres", Name: "_share_1234567890"},
			{Engine: "redis", Name: "_share_1234567890"},
		},
		SizeBytes: 1048576,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}

	var restored BundleManifest
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal manifest: %v", err)
	}

	if restored.ID != original.ID {
		t.Errorf("ID: got %q, want %q", restored.ID, original.ID)
	}
	if restored.Mode != original.Mode {
		t.Errorf("Mode: got %q, want %q", restored.Mode, original.Mode)
	}
	if restored.CheckpointName != original.CheckpointName {
		t.Errorf("CheckpointName: got %q, want %q", restored.CheckpointName, original.CheckpointName)
	}
	if len(restored.Containers) != len(original.Containers) {
		t.Errorf("Containers: got %d, want %d", len(restored.Containers), len(original.Containers))
	}
	if len(restored.Databases) != len(original.Databases) {
		t.Errorf("Databases: got %d, want %d", len(restored.Databases), len(original.Databases))
	}
	if restored.SizeBytes != original.SizeBytes {
		t.Errorf("SizeBytes: got %d, want %d", restored.SizeBytes, original.SizeBytes)
	}
}

func TestBundleManifestDbOnly(t *testing.T) {
	m := BundleManifest{
		ID:   "db-only-id",
		Mode: "db-only",
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Containers should be omitted (omitempty)
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["containers"]; ok {
		t.Error("expected containers field to be omitted in db-only mode")
	}
}

func TestTarGzRoundTrip(t *testing.T) {
	tmp := t.TempDir()

	// Create a source directory with nested files
	srcDir := filepath.Join(tmp, "source")
	subDir := filepath.Join(srcDir, "nested")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "deep.txt"), []byte("deep content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Tar+gzip it
	archivePath := filepath.Join(tmp, "test.tar.gz")
	if err := tarGzDirectory(srcDir, archivePath); err != nil {
		t.Fatalf("tarGzDirectory failed: %v", err)
	}

	// Verify archive exists and is non-empty
	fi, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("archive not created: %v", err)
	}
	if fi.Size() == 0 {
		t.Fatal("archive is empty")
	}

	// Extract to a new directory
	extractDir := filepath.Join(tmp, "extracted")
	if err := untarGz(archivePath, extractDir); err != nil {
		t.Fatalf("untarGz failed: %v", err)
	}

	// Verify root file
	rootContent, err := os.ReadFile(filepath.Join(extractDir, "root.txt"))
	if err != nil {
		t.Fatalf("failed to read extracted root.txt: %v", err)
	}
	if string(rootContent) != "root content" {
		t.Errorf("root.txt: got %q, want %q", string(rootContent), "root content")
	}

	// Verify nested file
	deepContent, err := os.ReadFile(filepath.Join(extractDir, "nested", "deep.txt"))
	if err != nil {
		t.Fatalf("failed to read extracted nested/deep.txt: %v", err)
	}
	if string(deepContent) != "deep content" {
		t.Errorf("deep.txt: got %q, want %q", string(deepContent), "deep content")
	}
}

func TestShareDir(t *testing.T) {
	// Default path
	dir := ShareDir()
	if dir == "" {
		t.Error("ShareDir() returned empty string")
	}

	// Override via env
	t.Setenv("DEVX_SHARE_DIR", "/tmp/test-share")
	if got := ShareDir(); got != "/tmp/test-share" {
		t.Errorf("ShareDir() with env override: got %q, want %q", got, "/tmp/test-share")
	}
}

func TestCleanupShareDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DEVX_SHARE_DIR", tmp)

	bundleID := "cleanup-test"
	bundleDir := filepath.Join(tmp, bundleID)
	_ = os.MkdirAll(bundleDir, 0755)
	_ = os.WriteFile(filepath.Join(bundleDir, "file.txt"), []byte("data"), 0644)
	_ = os.WriteFile(filepath.Join(tmp, bundleID+".bundle.tar.gz"), []byte("archive"), 0644)
	_ = os.WriteFile(filepath.Join(tmp, bundleID+".encrypted"), []byte("cipher"), 0644)

	CleanupShareDir(bundleID)

	if _, err := os.Stat(bundleDir); !os.IsNotExist(err) {
		t.Error("expected bundle directory to be removed")
	}
	if _, err := os.Stat(filepath.Join(tmp, bundleID+".bundle.tar.gz")); !os.IsNotExist(err) {
		t.Error("expected .bundle.tar.gz to be removed")
	}
	if _, err := os.Stat(filepath.Join(tmp, bundleID+".encrypted")); !os.IsNotExist(err) {
		t.Error("expected .encrypted to be removed")
	}
}
