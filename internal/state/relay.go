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
	"os/exec"
	"strings"
)

// ParseRelay determines the backend type and normalized URI from a config string.
// Currently supports s3:// and gs:// prefixes.
// TODO(Idea 56): Implement HTTP relay support (e.g., https://transfer.sh) as a fallback
// for users without cloud credentials.
func ParseRelay(relayConfig string) (backend string, uri string, err error) {
	relayConfig = strings.TrimSpace(relayConfig)
	if relayConfig == "" {
		return "", "", fmt.Errorf("no relay configured. Please set state.relay in devx.yaml or pass --relay")
	}

	if strings.HasPrefix(relayConfig, "s3://") {
		return "s3", relayConfig, nil
	}
	if strings.HasPrefix(relayConfig, "gs://") {
		return "gcs", relayConfig, nil
	}

	// TODO(Idea 56): Support HTTP URLs here (backend="http")
	return "", "", fmt.Errorf("unsupported relay format: %q. Must start with s3:// or gs://", relayConfig)
}

// UploadToS3 uploads a file using the AWS CLI.
func UploadToS3(filePath, s3URI string) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("S3 relay configured but 'aws' CLI not found. Please install it")
	}
	cmd := exec.Command("aws", "s3", "cp", filePath, s3URI)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to upload to S3: %w\n%s", err, string(out))
	}
	return nil
}

// DownloadFromS3 downloads a file using the AWS CLI.
func DownloadFromS3(s3URI, outputPath string) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("S3 relay configured but 'aws' CLI not found. Please install it")
	}
	cmd := exec.Command("aws", "s3", "cp", s3URI, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to download from S3: %w\n%s", err, string(out))
	}
	return nil
}

// UploadToGCS uploads a file using the Google Cloud CLI.
func UploadToGCS(filePath, gsURI string) error {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("GCS relay configured but 'gcloud' CLI not found. Please install it")
	}
	cmd := exec.Command("gcloud", "storage", "cp", filePath, gsURI)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to upload to GCS: %w\n%s", err, string(out))
	}
	return nil
}

// DownloadFromGCS downloads a file using the Google Cloud CLI.
func DownloadFromGCS(gsURI, outputPath string) error {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("GCS relay configured but 'gcloud' CLI not found. Please install it")
	}
	cmd := exec.Command("gcloud", "storage", "cp", gsURI, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to download from GCS: %w\n%s", err, string(out))
	}
	return nil
}
