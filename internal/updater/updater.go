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

// Package updater provides a lightweight, cached version check against GitHub Releases.
// It is designed to be non-blocking: fire it in a goroutine at startup and only
// print a notice when the command finishes, if a newer version was found.
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	repo          = "VitruvianSoftware/devx"
	checkInterval = 24 * time.Hour
	apiURL        = "https://api.github.com/repos/" + repo + "/releases/latest"
	timeout       = 5 * time.Second
)

type cacheFile struct {
	CheckedAt  time.Time `json:"checked_at"`
	LatestTag  string    `json:"latest_tag"`
	ReleaseURL string    `json:"release_url"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckResult holds the outcome of a version comparison.
type CheckResult struct {
	Current         string
	Latest          string
	ReleaseURL      string
	UpdateAvailable bool
}

// Check compares the running version against the latest GitHub Release.
// It caches the result for 24h to avoid rate-limiting. Safe to call concurrently.
func Check(current string) (*CheckResult, error) {
	// Never check during development builds
	if current == "dev" || current == "" {
		return &CheckResult{Current: current}, nil
	}

	cached, err := readCache()
	if err == nil && time.Since(cached.CheckedAt) < checkInterval {
		// Cache is fresh — use it
		return compare(current, cached.LatestTag, cached.ReleaseURL), nil
	}

	// Fetch fresh from GitHub
	latest, releaseURL, err := fetchLatest()
	if err != nil {
		// Network errors are non-fatal — just silently skip
		return &CheckResult{Current: current}, nil //nolint:nilerr
	}

	_ = writeCache(cacheFile{
		CheckedAt:  time.Now(),
		LatestTag:  latest,
		ReleaseURL: releaseURL,
	})

	return compare(current, latest, releaseURL), nil
}

// CheckForUpgrade always fetches the latest release from GitHub, bypassing both
// the 24h cache and the dev-build guard. Used by 'devx upgrade' so that the
// command always has fresh data and works during development.
func CheckForUpgrade(current string) (*CheckResult, error) {
	latest, releaseURL, err := fetchLatest()
	if err != nil {
		return nil, err
	}
	_ = writeCache(cacheFile{
		CheckedAt:  time.Now(),
		LatestTag:  latest,
		ReleaseURL: releaseURL,
	})
	return compare(current, latest, releaseURL), nil
}

func compare(current, latest, releaseURL string) *CheckResult {
	r := &CheckResult{
		Current:    current,
		Latest:     latest,
		ReleaseURL: releaseURL,
	}
	if latest == "" {
		return r
	}
	// Normalise: strip leading "v" for comparison
	cur := strings.TrimPrefix(current, "v")
	lat := strings.TrimPrefix(latest, "v")
	r.UpdateAvailable = cur != lat && lat > cur
	return r
}

func fetchLatest() (tag, htmlURL string, err error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", err
	}
	return rel.TagName, rel.HTMLURL, nil
}

func cacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devx")
}

func cachePath() string {
	return filepath.Join(cacheDir(), "update-check.json")
}

func readCache() (*cacheFile, error) {
	raw, err := os.ReadFile(cachePath())
	if err != nil {
		return nil, err
	}
	var c cacheFile
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func writeCache(c cacheFile) error {
	_ = os.MkdirAll(cacheDir(), 0755)
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(), raw, 0644)
}
