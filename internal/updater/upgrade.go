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

package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const releaseBaseURL = "https://github.com/" + repo + "/releases/download"

// DownloadURL returns the tarball URL for a given release tag on this platform.
func DownloadURL(tag string) string {
	return fmt.Sprintf("%s/%s/devx_%s_%s.tar.gz", releaseBaseURL, tag, runtime.GOOS, runtime.GOARCH)
}

// ChecksumURL returns the checksums.txt URL for a given release tag.
func ChecksumURL(tag string) string {
	return fmt.Sprintf("%s/%s/checksums.txt", releaseBaseURL, tag)
}

// SelfUpgrade downloads the given release tag, verifies the SHA-256 checksum,
// then atomically replaces the currently running binary.
// Progress is reported via the provided writer (use os.Stderr for terminal output).
func SelfUpgrade(tag string, progress io.Writer) error {
	// Resolve the real path of the running binary (follow symlinks)
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate current binary: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("could not resolve binary path: %w", err)
	}

	// Check write permission to the binary's directory early — avoids a
	// half-complete upgrade if the user forgot sudo.
	execDir := filepath.Dir(execPath)
	if err := checkWritable(execDir); err != nil {
		return fmt.Errorf("cannot write to %s: %w\n\nTip: try running with sudo: sudo devx upgrade", execDir, err)
	}

	tarURL := DownloadURL(tag)
	csumsURL := ChecksumURL(tag)

	_, _ = fmt.Fprintf(progress, "  → Downloading %s...\n", tarURL)
	tarBytes, err := downloadBytes(tarURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify SHA-256 checksum against the release's checksums.txt
	_, _ = fmt.Fprintf(progress, "  → Verifying checksum...\n")
	expectedSum, err := fetchExpectedChecksum(csumsURL, filepath.Base(tarURL))
	if err != nil {
		// Non-fatal: some older releases may not have a checksums file
		_, _ = fmt.Fprintf(progress, "  ⚠ Checksum verification skipped: %v\n", err)
	} else {
		actualSum := fmt.Sprintf("%x", sha256.Sum256(tarBytes))
		if actualSum != expectedSum {
			return fmt.Errorf("checksum mismatch — aborting to protect your system\n  expected: %s\n  got:      %s", expectedSum, actualSum)
		}
		_, _ = fmt.Fprintf(progress, "  ✓ Checksum verified.\n")
	}

	// Extract the devx binary from the tar.gz
	_, _ = fmt.Fprintf(progress, "  → Extracting binary...\n")
	newBinary, err := extractBinary(tarBytes, "devx")
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Write to a temp file in the same directory as the current binary.
	// Same filesystem guarantees os.Rename is atomic (no cross-device copy).
	tmpFile, err := os.CreateTemp(execDir, ".devx-upgrade-*")
	if err != nil {
		return fmt.Errorf("could not create temp file in %s: %w\n\nTip: try running with sudo: sudo devx upgrade", execDir, err)
	}
	tmpPath := tmpFile.Name()
	// Always clean up the temp file on any failure path
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.Write(newBinary); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("could not write new binary: %w", err)
	}
	if err := tmpFile.Chmod(0755); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("could not chmod new binary: %w", err)
	}
	_ = tmpFile.Close()

	// Atomic replace — on Unix, os.Rename works even when the binary is running.
	// The old inode stays alive until the OS process closes it naturally.
	_, _ = fmt.Fprintf(progress, "  → Installing to %s...\n", execPath)
	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("could not replace %s: %w\n\nTip: try running with sudo: sudo devx upgrade", execPath, err)
	}

	return nil
}

// checkWritable probes whether we can create files in the given directory.
func checkWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".devx-write-test-*")
	if err != nil {
		return err
	}
	_ = f.Close()
	return os.Remove(f.Name())
}

// downloadBytes fetches a URL and returns the full response body as bytes.
func downloadBytes(url string) ([]byte, error) {
	client := &http.Client{}     // no timeout — downloads can be slow on poor connections
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() 
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

// fetchExpectedChecksum downloads the checksums.txt file and returns the
// SHA-256 hex string for the given filename.
func fetchExpectedChecksum(url, filename string) (string, error) {
	data, err := downloadBytes(url)
	if err != nil {
		return "", err
	}
	// checksums.txt format: "<sha256hex>  <filename>\n"
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filename {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %q not found in checksums.txt", filename)
}

// extractBinary reads a .tar.gz payload from raw bytes and returns the
// raw bytes of the file named binaryName inside (matched on basename).
func extractBinary(tarGzBytes []byte, binaryName string) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(tarGzBytes))
	if err != nil {
		return nil, fmt.Errorf("not a valid gzip archive: %w", err)
	}
	defer func() { _ = gr.Close() }() 

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading archive: %w", err)
		}
		// Match just the filename component — archive may have a directory prefix
		if filepath.Base(hdr.Name) == binaryName {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%q not found inside the archive", binaryName)
}
