package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/VitruvianSoftware/devx/internal/updater"
	"github.com/spf13/cobra"
)

// version is set at build time via ldflags.
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the devx version and check for updates",
	Run: func(cmd *cobra.Command, args []string) {
		result, _ := updater.Check(version)

		if outputJSON {
			type versionJSON struct {
				Version         string `json:"version"`
				Latest          string `json:"latest,omitempty"`
				UpdateAvailable bool   `json:"update_available"`
				ReleaseURL      string `json:"release_url,omitempty"`
				OS              string `json:"os"`
				Arch            string `json:"arch"`
			}
			out := versionJSON{
				Version: version,
				OS:      runtime.GOOS,
				Arch:    runtime.GOARCH,
			}
			if result != nil {
				out.Latest = result.Latest
				out.UpdateAvailable = result.UpdateAvailable
				out.ReleaseURL = result.ReleaseURL
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			fmt.Println(string(b))
			return
		}

		fmt.Printf("devx %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)

		if result == nil || result.Latest == "" {
			return
		}

		if result.UpdateAvailable {
			fmt.Printf("\n✦ devx %s is available!\n", result.Latest)
			fmt.Printf("  %s\n\n", result.ReleaseURL)
			fmt.Printf("  Run: devx upgrade\n")
		} else {
			fmt.Printf("✓ You are up to date.\n")
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

