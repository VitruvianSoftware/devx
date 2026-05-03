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
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/database"
)

// BundleManifest describes the contents of a state replication bundle.
type BundleManifest struct {
	ID             string                  `json:"id"`
	Mode           string                  `json:"mode"` // "full" or "db-only"
	CheckpointName string                  `json:"checkpoint_name"`
	Containers     []string                `json:"containers,omitempty"` // Filenames of CRIU archives
	Databases      []database.SnapshotMeta `json:"databases,omitempty"`
	SizeBytes      int64                   `json:"size_bytes"`
}

// BundleResult holds the result of a bundling operation.
type BundleResult struct {
	ArchivePath string
	Manifest    BundleManifest
}

// ShareDir returns the base directory for state sharing operations.
func ShareDir() string {
	if d := os.Getenv("DEVX_SHARE_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devx", "share")
}

// BundleState bundles CRIU checkpoints and database snapshots into a single tar.gz archive.
func BundleState(checkpointName, bundleID string, dbSnapshots []database.SnapshotMeta, fullMode bool) (*BundleResult, error) {
	bundleDir := filepath.Join(ShareDir(), bundleID)
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create bundle directory: %w", err)
	}

	manifest := BundleManifest{
		ID:             bundleID,
		Mode:           "db-only",
		CheckpointName: checkpointName,
		Databases:      dbSnapshots,
	}

	if fullMode {
		manifest.Mode = "full"
		// Copy CRIU archives
		cpDir := checkpointPath(checkpointName)
		archives, err := filepath.Glob(filepath.Join(cpDir, "*.tar.gz"))
		if err == nil && len(archives) > 0 {
			containersDir := filepath.Join(bundleDir, "containers")
			if err := os.MkdirAll(containersDir, 0755); err != nil {
				return nil, err
			}
			for _, arch := range archives {
				base := filepath.Base(arch)
				manifest.Containers = append(manifest.Containers, base)
				if err := copyFile(arch, filepath.Join(containersDir, base)); err != nil {
					return nil, fmt.Errorf("failed to copy CRIU archive: %w", err)
				}
			}
		}
	}

	// Copy DB snapshots
	if len(dbSnapshots) > 0 {
		dbDir := filepath.Join(bundleDir, "databases")
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, err
		}
		for _, db := range dbSnapshots {
			// Copy .tar
			srcTar := filepath.Join(database.SnapshotDir(), db.Engine, db.Name+".tar")
			if err := copyFile(srcTar, filepath.Join(dbDir, fmt.Sprintf("%s_%s.tar", db.Engine, db.Name))); err != nil {
				return nil, fmt.Errorf("failed to copy db snapshot tar: %w", err)
			}
			// Copy .json
			srcJson := filepath.Join(database.SnapshotDir(), db.Engine, db.Name+".json")
			if err := copyFile(srcJson, filepath.Join(dbDir, fmt.Sprintf("%s_%s.json", db.Engine, db.Name))); err != nil {
				return nil, fmt.Errorf("failed to copy db snapshot json: %w", err)
			}
		}
	}

	// Write manifest.json
	mfBytes, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(bundleDir, "manifest.json"), mfBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	// Tar and gzip the directory
	archivePath := filepath.Join(ShareDir(), bundleID+".bundle.tar.gz")
	if err := tarGzDirectory(bundleDir, archivePath); err != nil {
		return nil, fmt.Errorf("failed to compress bundle: %w", err)
	}

	fi, err := os.Stat(archivePath)
	if err == nil {
		manifest.SizeBytes = fi.Size()
	}

	return &BundleResult{
		ArchivePath: archivePath,
		Manifest:    manifest,
	}, nil
}

// UnbundleState extracts a bundle archive to a temporary directory and parses its manifest.
func UnbundleState(archivePath, extractDir string) (*BundleManifest, error) {
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create extraction directory: %w", err)
	}

	if err := untarGz(archivePath, extractDir); err != nil {
		return nil, fmt.Errorf("failed to extract bundle: %w", err)
	}

	mfBytes, err := os.ReadFile(filepath.Join(extractDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("bundle is missing manifest.json: %w", err)
	}

	var manifest BundleManifest
	if err := json.Unmarshal(mfBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest.json: %w", err)
	}

	return &manifest, nil
}

// CleanupShareDir removes artifacts in the share directory for a specific ID.
func CleanupShareDir(bundleID string) {
	_ = os.RemoveAll(filepath.Join(ShareDir(), bundleID))
	_ = os.Remove(filepath.Join(ShareDir(), bundleID+".bundle.tar.gz"))
	_ = os.Remove(filepath.Join(ShareDir(), bundleID+".encrypted"))
}

// --- Helpers ---

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func tarGzDirectory(srcDir, destArchive string) error {
	out, err := os.Create(destArchive)
	if err != nil {
		return err
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// Use relative path for header name
		relPath, err := filepath.Rel(srcDir, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

func untarGz(srcArchive, destDir string) error {
	in, err := os.Open(srcArchive)
	if err != nil {
		return err
	}
	defer in.Close()

	gr, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		// Basic zip slip prevention
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != destDir {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}
