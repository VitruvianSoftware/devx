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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/updater"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade devx to the latest release",
	Long: `Downloads the latest devx release from GitHub, verifies the SHA-256 checksum,
and atomically replaces the currently running binary in-place.

If devx is installed in a system directory (e.g. /usr/local/bin), you may
need to run with sudo:

  sudo devx upgrade`,
	RunE: runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(_ *cobra.Command, _ []string) error {
	fmt.Printf("Checking for updates (current: %s)...\n\n", version)

	// upgrades always do a fresh GitHub check, even for dev builds
	result, err := updater.CheckForUpgrade(version)
	if err != nil {
		return fmt.Errorf("version check failed: %w", err)
	}

	if result == nil || result.Latest == "" {
		return fmt.Errorf("could not determine latest version — check your network connection")
	}

	if !result.UpdateAvailable {
		fmt.Printf("✓ devx %s is already the latest version.", version)
		if isGoInstallBinary() {
			fmt.Printf("\n\n  Installed via: go install\n  To upgrade: go install github.com/VitruvianSoftware/devx@latest\n")
		}
		fmt.Println()
		return nil
	}

	fmt.Printf("  Current: %s\n", result.Current)
	fmt.Printf("  Latest:  %s\n", result.Latest)
	fmt.Println()

	if !NonInteractive {
		fmt.Printf("Upgrade devx %s → %s? [y/N] ", result.Current, result.Latest)
		var confirm string
		fmt.Scanln(&confirm) //nolint:errcheck
		if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
			fmt.Println("Aborted.")
			return nil
		}
		fmt.Println()
	}

	if DryRun {
		fmt.Printf("[dry-run] Would download and install devx %s from:\n  %s\n",
			result.Latest, updater.DownloadURL(result.Latest))
		return nil
	}

	fmt.Printf("Upgrading devx %s → %s\n\n", result.Current, result.Latest)

	if err := updater.SelfUpgrade(result.Latest, os.Stderr); err != nil {
		return err
	}

	fmt.Printf("\n✅ devx upgraded to %s successfully!\n", result.Latest)

	if outputJSON {
		fmt.Printf("{\"upgraded\": true, \"version\": %q}\n", result.Latest)
	}

	return nil
}

// isGoInstallBinary returns true when devx appears to have been installed via
// 'go install' by checking if the binary lives inside a known Go bin directory
// (GOPATH/bin or ~/go/bin).
func isGoInstallBinary() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	// Prefer GOPATH env var, fall back to the conventional ~/go default
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	goBin := filepath.Join(gopath, "bin")

	return strings.HasPrefix(filepath.Clean(execPath), filepath.Clean(goBin))
}
