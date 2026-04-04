package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const grafanaAPI = "http://localhost:3000/api/dashboards/db"

// ProvisionDashboard auto-installs the "devx Build Metrics" dashboard into a
// running Grafana instance. Fire-and-forget: warns on error but never fatals.
func ProvisionDashboard() error {
	payload := map[string]interface{}{
		"dashboard": dashboardJSON(),
		"overwrite": true,
		"message":   "Auto-provisioned by devx trace spawn",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling dashboard: %w", err)
	}

	req, err := http.NewRequest("POST", grafanaAPI, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("admin", "admin")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("posting dashboard: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("grafana API returned %d", resp.StatusCode)
	}

	return nil
}

// dashboardJSON returns the Grafana dashboard model for devx build metrics.
// Aggregate metric panels use Prometheus (via Tempo's span-metrics generator).
// Table/search panels use Tempo directly with TraceQL.
func dashboardJSON() map[string]interface{} {
	return map[string]interface{}{
		"id":            nil,
		"uid":           "devx-build-metrics",
		"title":         "devx Build Metrics",
		"tags":          []string{"devx", "ci", "build"},
		"timezone":      "browser",
		"schemaVersion": 39,
		"version":       1,
		"refresh":       "30s",
		"time": map[string]interface{}{
			"from": "now-7d",
			"to":   "now",
		},
		"annotations": map[string]interface{}{
			"list": []interface{}{},
		},
		"panels": []interface{}{
			// ── Row 1: Overview stats (Prometheus) ────────────────────
			promStatPanel("Total Builds", 0, 0, 6, 4,
				`sum(traces_spanmetrics_calls_total{service="devx", span_name=~"devx_run|agent_ship_build"})`,
				"lastNotNull", "#79C0FF", "short"),
			promStatPanel("Total Preflights", 6, 0, 6, 4,
				`sum(traces_spanmetrics_calls_total{service="devx", span_name="agent_ship_preflight"})`,
				"lastNotNull", "#A5D6FF", "short"),
			promStatPanel("Avg Build Time", 12, 0, 6, 4,
				`sum(traces_spanmetrics_latency_sum{service="devx", span_name=~"devx_run|agent_ship_build"}) / sum(traces_spanmetrics_latency_count{service="devx", span_name=~"devx_run|agent_ship_build"})`,
				"lastNotNull", "#7EE787", "s"),
			promStatPanel("Total Tests", 18, 0, 6, 4,
				`sum(traces_spanmetrics_calls_total{service="devx", span_name="go_test"})`,
				"lastNotNull", "#FFA657", "short"),

			// ── Row 2: Build duration over time + Recent commands ─────
			promTimeSeriesPanel("Build Duration Over Time", 0, 4, 12, 8,
				`sum by (span_name) (rate(traces_spanmetrics_latency_sum{service="devx", span_name=~"devx_run|agent_ship_build"}[5m])) / sum by (span_name) (rate(traces_spanmetrics_latency_count{service="devx", span_name=~"devx_run|agent_ship_build"}[5m]))`,
				"s"),
			tablePanel("Recent Commands (devx run)", 12, 4, 12, 8,
				`{resource.service.name="devx" && name="devx_run"}`),

			// ── Row 3: Span counts by type (Prometheus bar gauge) ─────
			promBarGaugePanel("Build & Run Activity", 0, 12, 8, 6, []promTarget{
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="devx_run"})`, Legend: "devx run", RefID: "A"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="agent_ship_build"})`, Legend: "agent ship build", RefID: "B"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="agent_ship_preflight"})`, Legend: "agent ship preflight", RefID: "C"},
			}),
			promBarGaugePanel("Test Activity", 8, 12, 8, 6, []promTarget{
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="go_test"})`, Legend: "go_test spans", RefID: "A"},
			}),
			promTimeSeriesPanel("Test Execution Rate", 16, 12, 8, 6,
				`sum(rate(traces_spanmetrics_calls_total{service="devx", span_name="go_test"}[5m]))`,
				"cps"),

			// ── Row 4: Recent Preflights & Test Details (Tempo tables) ──
			tablePanel("Recent Agent Ship Preflights", 0, 18, 12, 8,
				`{resource.service.name="devx" && name="agent_ship_preflight"}`),
			tablePanelWithSelect("Test Details", 12, 18, 12, 8,
				`{resource.service.name="devx" && name="go_test"} | select(span.devx.test.name, span.devx.test.status, span.devx.test.package)`),
		},
	}
}

// ── Prometheus Panel Builders ───────────────────────────────────────────────

type promTarget struct {
	Query  string
	Legend string
	RefID  string
}

func promStatPanel(title string, x, y, w, h int, query, reducer, color, unit string) map[string]interface{} {
	return map[string]interface{}{
		"type":  "stat",
		"title": title,
		"gridPos": map[string]int{
			"x": x, "y": y, "w": w, "h": h,
		},
		"datasource": map[string]string{
			"type": "prometheus",
			"uid":  "prometheus",
		},
		"targets": []map[string]interface{}{
			{
				"datasource": map[string]string{"type": "prometheus", "uid": "prometheus"},
				"expr":       query,
				"refId":      "A",
				"instant":    true,
			},
		},
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]string{"fixedColor": color, "mode": "fixed"},
				"unit":  unit,
			},
		},
		"options": map[string]interface{}{
			"colorMode": "background",
			"graphMode": "area",
			"textMode":  "auto",
			"reduceOptions": map[string]interface{}{
				"calcs": []string{reducer},
			},
		},
	}
}

func promTimeSeriesPanel(title string, x, y, w, h int, query, unit string) map[string]interface{} {
	return map[string]interface{}{
		"type":  "timeseries",
		"title": title,
		"gridPos": map[string]int{
			"x": x, "y": y, "w": w, "h": h,
		},
		"datasource": map[string]string{
			"type": "prometheus",
			"uid":  "prometheus",
		},
		"targets": []map[string]interface{}{
			{
				"datasource": map[string]string{"type": "prometheus", "uid": "prometheus"},
				"expr":       query,
				"refId":      "A",
			},
		},
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"unit": unit,
				"custom": map[string]interface{}{
					"drawStyle":   "line",
					"lineWidth":   2,
					"fillOpacity": 15,
					"pointSize":   5,
					"showPoints":  "auto",
				},
			},
		},
	}
}

func promBarGaugePanel(title string, x, y, w, h int, targets []promTarget) map[string]interface{} {
	queryTargets := make([]map[string]interface{}, 0, len(targets))
	for _, t := range targets {
		queryTargets = append(queryTargets, map[string]interface{}{
			"datasource":   map[string]string{"type": "prometheus", "uid": "prometheus"},
			"expr":         t.Query,
			"legendFormat": t.Legend,
			"refId":        t.RefID,
			"instant":      true,
		})
	}

	return map[string]interface{}{
		"type":  "bargauge",
		"title": title,
		"gridPos": map[string]int{
			"x": x, "y": y, "w": w, "h": h,
		},
		"datasource": map[string]string{
			"type": "prometheus",
			"uid":  "prometheus",
		},
		"targets": queryTargets,
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]string{
					"mode": "palette-classic",
				},
			},
		},
		"options": map[string]interface{}{
			"orientation": "horizontal",
			"displayMode": "gradient",
			"reduceOptions": map[string]interface{}{
				"calcs": []string{"lastNotNull"},
			},
		},
	}
}

// ── Tempo Panel Builders ────────────────────────────────────────────────────

func tablePanel(title string, x, y, w, h int, query string) map[string]interface{} {
	return map[string]interface{}{
		"type":  "table",
		"title": title,
		"gridPos": map[string]int{
			"x": x, "y": y, "w": w, "h": h,
		},
		"datasource": map[string]string{
			"type": "tempo",
			"uid":  "tempo",
		},
		"targets": []map[string]interface{}{
			{
				"datasource": map[string]string{"type": "tempo", "uid": "tempo"},
				"queryType":  "traceql",
				"query":      query,
				"tableType":  "spans",
				"limit":      20,
			},
		},
		"options": map[string]interface{}{
			"showHeader": true,
			"sortBy": []map[string]interface{}{
				{"displayName": "Start time", "desc": true},
			},
		},
	}
}

func tablePanelWithSelect(title string, x, y, w, h int, query string) map[string]interface{} {
	return map[string]interface{}{
		"type":  "table",
		"title": title,
		"gridPos": map[string]int{
			"x": x, "y": y, "w": w, "h": h,
		},
		"datasource": map[string]string{
			"type": "tempo",
			"uid":  "tempo",
		},
		"targets": []map[string]interface{}{
			{
				"datasource": map[string]string{"type": "tempo", "uid": "tempo"},
				"queryType":  "traceql",
				"query":      query,
				"tableType":  "spans",
				"limit":      50,
			},
		},
		"options": map[string]interface{}{
			"showHeader": true,
			"sortBy": []map[string]interface{}{
				{"displayName": "Start time", "desc": true},
			},
		},
	}
}
