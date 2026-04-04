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
// Datasource is set to Tempo (the default in the grafana/otel-lgtm stack).
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
			// Row 1: Overview stats
			statPanel("Total Builds", 0, 0, 6, 4,
				`{resource.service.name="devx" && name="agent_ship_build"}`,
				"count", "#79C0FF"),
			statPanel("Total Preflights", 6, 0, 6, 4,
				`{resource.service.name="devx" && name="agent_ship_preflight"}`,
				"count", "#A5D6FF"),
			statPanel("P50 Build Time", 12, 0, 6, 4,
				`{resource.service.name="devx" && name="agent_ship_build"}`,
				"p50(duration)", "#7EE787"),
			statPanel("P90 Build Time", 18, 0, 6, 4,
				`{resource.service.name="devx" && name="agent_ship_build"}`,
				"p90(duration)", "#FFA657"),

			// Row 2: Build duration over time + Recent builds table
			timeSeriesPanel("Build Duration Over Time", 0, 4, 12, 8,
				`{resource.service.name="devx" && name="agent_ship_build"}`),
			tablePanel("Recent Builds", 12, 4, 12, 8,
				`{resource.service.name="devx" && name="agent_ship_preflight"}`),

			// Row 3: Pass/fail breakdown
			piePanel("Test Results", 0, 12, 8, 6,
				`{resource.service.name="devx" && name="agent_ship_preflight"}`,
				"span.devx.test.pass"),
			piePanel("Lint Results", 8, 12, 8, 6,
				`{resource.service.name="devx" && name="agent_ship_preflight"}`,
				"span.devx.lint.pass"),
			piePanel("Build Results", 16, 12, 8, 6,
				`{resource.service.name="devx" && name="agent_ship_preflight"}`,
				"span.devx.build.pass"),
		},
	}
}

// ── Panel Builders ──────────────────────────────────────────────────────────

func statPanel(title string, x, y, w, h int, query, aggregate, color string) map[string]interface{} {
	return map[string]interface{}{
		"type":  "stat",
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
				"queryType":  "traceqlmetrics",
				"query":      query,
				"tableType":  "spans",
			},
		},
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"color": map[string]string{"fixedColor": color, "mode": "fixed"},
				"unit":  "ms",
			},
		},
		"options": map[string]interface{}{
			"colorMode":   "background",
			"graphMode":   "area",
			"textMode":    "auto",
			"reduceOptions": map[string]interface{}{
				"calcs": []string{"lastNotNull"},
			},
		},
	}
}

func timeSeriesPanel(title string, x, y, w, h int, query string) map[string]interface{} {
	return map[string]interface{}{
		"type":  "timeseries",
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
				"queryType":  "traceqlmetrics",
				"query":      query,
				"tableType":  "spans",
			},
		},
		"fieldConfig": map[string]interface{}{
			"defaults": map[string]interface{}{
				"unit": "ms",
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

func piePanel(title string, x, y, w, h int, query, groupBy string) map[string]interface{} {
	return map[string]interface{}{
		"type":  "piechart",
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
			},
		},
		"options": map[string]interface{}{
			"pieType":     "donut",
			"reduceOptions": map[string]interface{}{
				"calcs": []string{"lastNotNull"},
			},
			"legend": map[string]interface{}{
				"displayMode": "list",
				"placement":   "right",
			},
		},
	}
}
