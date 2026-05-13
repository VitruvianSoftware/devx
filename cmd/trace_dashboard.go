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

	"github.com/VitruvianSoftware/devx/internal/telemetry"
	"github.com/spf13/cobra"
)

var traceDashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Provision the devx Build Metrics dashboard to the configured Grafana instance",
	Long: `Pushes the default devx Build Metrics dashboard to the Grafana URL
configured in devx.yaml or the DEVX_GRAFANA_URL environment variable.

This is useful if you are using an external Grafana instance (like one
running in your cluster) instead of the local one spawned by 'devx trace spawn'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Attempt to load config to populate telemetry endpoints
		// This will set the grafana URL globally if configured in devx.yaml
		_, _ = resolveConfig("devx.yaml", "")

		fmt.Println("🚀 Provisioning devx Build Metrics dashboard...")

		if err := telemetry.ProvisionDashboard(); err != nil {
			return fmt.Errorf("failed to provision dashboard: %w", err)
		}

		fmt.Println("✅ Dashboard successfully provisioned to Grafana!")
		return nil
	},
}

func init() {
	traceCmd.AddCommand(traceDashboardCmd)
}
