package cmd

import (
	"fmt"
	"os"
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
		fmt.Printf("✓ devx %s is already the latest version.\n", version)
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
