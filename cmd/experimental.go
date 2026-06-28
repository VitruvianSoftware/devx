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
	"strings"

	"github.com/spf13/cobra"
)

// Experimental is set by --experimental; together with the DEVX_EXPERIMENTAL env
// var it unlocks the Tier-3 / alpha commands that ride brittle third-party tools,
// so the GA core's reliability isn't diluted by them being invoked by accident.
// See docs/analysis/feature_maturity_analysis.md.
var Experimental bool

// experimentalEnabled reports whether alpha commands are unlocked — via the
// --experimental flag or a truthy DEVX_EXPERIMENTAL env var.
func experimentalEnabled() bool {
	if Experimental {
		return true
	}
	switch strings.ToLower(os.Getenv("DEVX_EXPERIMENTAL")) {
	case "", "0", "false", "no":
		return false
	default:
		return true
	}
}

// markExperimental gates a command (and its subcommands, via PersistentPreRunE)
// behind --experimental / DEVX_EXPERIMENTAL and tags its help label, so an alpha
// feature on a brittle dependency can't be run by accident.
func markExperimental(cmd *cobra.Command, feature string) {
	const tag = " [experimental]"
	if !strings.HasSuffix(cmd.Short, tag) {
		cmd.Short += tag
	}
	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if experimentalEnabled() {
			return nil
		}
		return fmt.Errorf("%s is experimental (alpha): re-run with --experimental or set DEVX_EXPERIMENTAL=1 to enable it", feature)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&Experimental, "experimental", false,
		"Enable experimental (alpha) commands that depend on brittle third-party tools")

	// Tier-3 / alpha features per docs/analysis/feature_maturity_analysis.md: local
	// cloud emulators (imperfect), zero-config k3s-in-container (nested-net quirks),
	// OpenAPI mocking (Prism crashes on complex specs), and the local email catcher
	// (MailHog, deprecated upstream). The multi-node `cluster` suite and the hybrid
	// `bridge` are intentionally NOT gated — they are in active use.
	markExperimental(cloudCmd, "devx cloud (local GCP/S3 emulators)")
	markExperimental(k8sCmd, "devx k8s (zero-config k3s-in-container)")
	markExperimental(mockCmd, "devx mock (OpenAPI mocking via Prism)")
	markExperimental(mailCmd, "devx mail (local email catcher)")
}
