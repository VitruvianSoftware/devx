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
	"math"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/VitruvianSoftware/devx/internal/telemetry"
)

var statsClear bool

var statsCmd = &cobra.Command{
	Use:   "stats",
	GroupID: "telemetry",
	Short: "Display local devx performance metrics (P50/P90/P99 latency)",
	Long: `Reads the local metrics log at ~/.devx/metrics.json and displays
percentile latency for each recorded event type.

Metrics are recorded automatically during 'devx up' and 'devx agent ship'.

  devx stats          # human-readable table
  devx stats --json   # machine-readable output
  devx stats --clear  # truncate the metrics file`,
	RunE: runStats,
}

var (
	statsStyleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#79C0FF"))
	statsStyleHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8B949E"))
	statsStyleValue  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E6EDF3"))
	statsStyleMuted  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6E7681"))
)

// statsJSON is the machine-readable output format.
type statsJSON struct {
	Events []statsEventJSON `json:"events"`
	Total  int              `json:"total_entries"`
	Path   string           `json:"path"`
}

type statsEventJSON struct {
	Event string `json:"event"`
	Count int    `json:"count"`
	P50Ms int64  `json:"p50_ms"`
	P90Ms int64  `json:"p90_ms"`
	P99Ms int64  `json:"p99_ms"`
}

func runStats(_ *cobra.Command, _ []string) error {
	if statsClear {
		if !NonInteractive {
			var confirmed bool
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Clear all local devx metrics?").
						Description("This will delete all recorded performance data in ~/.devx/metrics.json.").
						Value(&confirmed),
				),
			)
			if err := form.Run(); err != nil {
				return err
			}
			if !confirmed {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		if err := telemetry.ClearMetrics(); err != nil {
			return fmt.Errorf("clearing metrics: %w", err)
		}
		fmt.Println("✓ Metrics cleared.")
		return nil
	}

	entries := telemetry.LoadMetrics()
	if len(entries) == 0 {
		fmt.Println("No metrics recorded yet. Run 'devx up' or 'devx agent ship' to start collecting data.")
		return nil
	}

	// Filter to last 30 days
	cutoff := time.Now().AddDate(0, 0, -30)
	var recent []telemetry.MetricEntry
	for _, e := range entries {
		ts, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil {
			continue
		}
		if ts.After(cutoff) {
			recent = append(recent, e)
		}
	}

	if len(recent) == 0 {
		fmt.Println("No metrics recorded in the last 30 days.")
		return nil
	}

	// Group by event
	grouped := make(map[string][]int64)
	for _, e := range recent {
		grouped[e.Event] = append(grouped[e.Event], e.DurationMs)
	}

	// Sort event names for deterministic output
	var eventNames []string
	for name := range grouped {
		eventNames = append(eventNames, name)
	}
	sort.Strings(eventNames)

	if outputJSON {
		out := statsJSON{
			Total: len(recent),
			Path:  "~/.devx/metrics.json",
		}
		for _, name := range eventNames {
			durations := grouped[name]
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			out.Events = append(out.Events, statsEventJSON{
				Event: name,
				Count: len(durations),
				P50Ms: percentile(durations, 50),
				P90Ms: percentile(durations, 90),
				P99Ms: percentile(durations, 99),
			})
		}
		enc, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(enc))
		return nil
	}

	// Human-readable output
	fmt.Println()
	fmt.Println(statsStyleTitle.Render("📊 devx local metrics (last 30 days)"))
	fmt.Println()

	// Table header
	fmt.Printf("  %-25s %7s %10s %10s %10s\n",
		statsStyleHeader.Render("Event"),
		statsStyleHeader.Render("Count"),
		statsStyleHeader.Render("P50"),
		statsStyleHeader.Render("P90"),
		statsStyleHeader.Render("P99"),
	)
	fmt.Printf("  %s\n", statsStyleMuted.Render(strings.Repeat("─", 65)))

	for _, name := range eventNames {
		durations := grouped[name]
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

		p50 := formatDuration(percentile(durations, 50))
		p90 := formatDuration(percentile(durations, 90))
		p99 := formatDuration(percentile(durations, 99))

		fmt.Printf("  %-25s %7d %10s %10s %10s\n",
			statsStyleValue.Render(name),
			len(durations),
			p50, p90, p99,
		)
	}

	fmt.Printf("\n  %s\n\n",
		statsStyleMuted.Render(fmt.Sprintf("Data: ~/.devx/metrics.json (%d entries)", len(recent))),
	)

	return nil
}

// percentile returns the p-th percentile value from a sorted slice.
func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(p)/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// formatDuration converts milliseconds to a human-readable string.
func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

func init() {
	statsCmd.Flags().BoolVar(&statsClear, "clear", false, "Truncate the local metrics file")
	rootCmd.AddCommand(statsCmd)
}
