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
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/VitruvianSoftware/devx/internal/updater"
	"github.com/spf13/cobra"
)

// version is set at build time via ldflags by GoReleaser.
// For 'go install' builds, we fall back to the Go module build info.
var version = "dev"

func init() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}
}

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
