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

package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

var (
	grafanaAPI    = "http://localhost:3000/api/dashboards/db"
	prometheusUID = "Prometheus-Data"
	tempoUID      = "Tempo"
)

func init() {
	if url := os.Getenv("DEVX_GRAFANA_URL"); url != "" {
		grafanaAPI = fmt.Sprintf("%s/api/dashboards/db", url)
	}
	if uid := os.Getenv("DEVX_GRAFANA_PROMETHEUS_UID"); uid != "" {
		prometheusUID = uid
	}
	if uid := os.Getenv("DEVX_GRAFANA_TEMPO_UID"); uid != "" {
		tempoUID = uid
	}
}

// SetGrafanaURL overrides the default or env-configured Grafana URL.
func SetGrafanaURL(url string) {
	if url != "" {
		grafanaAPI = fmt.Sprintf("%s/api/dashboards/db", url)
	}
}

func getGrafanaAPI() string {
	return grafanaAPI
}

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

	req, err := http.NewRequest("POST", getGrafanaAPI(), bytes.NewReader(body))
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
	defer func() { _ = resp.Body.Close() }()

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
			// ── Stage 1: Environment & Setup (devx up) ────────────────
			promStatPanel("Env Bootstrap P95", 0, 0, 6, 8,
				`histogram_quantile(0.95, sum(rate(traces_spanmetrics_latency_bucket{service="devx", span_name="up_startup"}[1h])) by (le))`,
				"lastNotNull", "#D2A8FF", "s"),
			promTimeSeriesPanel("Environment Bootstrap Latency", 6, 0, 18, 8,
				`histogram_quantile(0.95, sum(rate(traces_spanmetrics_latency_bucket{service="devx", span_name="up_startup"}[5m])) by (le))`,
				"s"),

			// ── Stage 2: Local Development (devx run / build) ─────────
			promStatPanel("Total Builds", 0, 8, 6, 8,
				`sum(traces_spanmetrics_calls_total{service="devx", span_name=~"devx_run|agent_ship_build"})`,
				"lastNotNull", "#79C0FF", "short"),
			promStatPanel("Avg Build Time", 6, 8, 6, 8,
				`sum(traces_spanmetrics_latency_sum{service="devx", span_name=~"devx_run|agent_ship_build"}) / sum(traces_spanmetrics_latency_count{service="devx", span_name=~"devx_run|agent_ship_build"})`,
				"lastNotNull", "#7EE787", "s"),
			promBarGaugePanel("Build & Run Activity", 12, 8, 12, 8, []promTarget{
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="devx_run"})`, Legend: "devx run", RefID: "A"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="devx_action"})`, Legend: "devx action", RefID: "B"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="agent_ship_build"})`, Legend: "agent ship build", RefID: "C"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="agent_ship_preflight"})`, Legend: "agent ship preflight", RefID: "D"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="up_startup"})`, Legend: "devx up startup", RefID: "E"},
			}),

			promTimeSeriesPanel("Build Duration Over Time", 0, 16, 12, 8,
				`sum by (span_name) (rate(traces_spanmetrics_latency_sum{service="devx", span_name=~"devx_run|agent_ship_build"}[5m])) / sum by (span_name) (rate(traces_spanmetrics_latency_count{service="devx", span_name=~"devx_run|agent_ship_build"}[5m]))`,
				"s"),
			tablePanel("Recent Commands (devx run)", 12, 16, 12, 8,
				`{resource.service.name="devx" && name="devx_run"}`),

			// ── Stage 3: Testing (go test) ────────────────────────────
			promStatPanel("Total Tests", 0, 24, 4, 8,
				`sum(traces_spanmetrics_calls_total{service="devx", span_name=~"go_test:.*"})`,
				"lastNotNull", "#FFA657", "short"),
			promBarGaugePanel("Test Activity", 4, 24, 8, 8, []promTarget{
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name=~"go_test:.*"})`, Legend: "total tests", RefID: "A"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name=~"go_test:.*", status_code="STATUS_CODE_UNSET"})`, Legend: "passed", RefID: "B"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name=~"go_test:.*", status_code="STATUS_CODE_ERROR"})`, Legend: "failed", RefID: "C"},
			}),
			promTimeSeriesPanel("Test Execution Rate", 12, 24, 6, 8,
				`sum(rate(traces_spanmetrics_calls_total{service="devx", span_name=~"go_test:.*"}[5m]))`,
				"cps"),
			promTimeSeriesPanel("Test Failure Rate", 18, 24, 6, 8,
				`sum(rate(traces_spanmetrics_calls_total{service="devx", span_name=~"go_test:.*", status_code="STATUS_CODE_ERROR"}[5m])) / sum(rate(traces_spanmetrics_calls_total{service="devx", span_name=~"go_test:.*"}[5m])) * 100`,
				"percent"),

			tablePanelWithSelect("Top Slowest Tests", 0, 32, 12, 8,
				`{resource.service.name="devx" && name=~"go_test:.*"} | select(span.devx.test.name, span.devx.test.package, span.devx.test.status)`),
			tablePanelWithSelect("Test Details", 12, 32, 12, 8,
				`{resource.service.name="devx" && name=~"go_test:.*"} | select(span.devx.test.name, span.devx.test.status, span.devx.test.package)`),

			// ── Stage 4: Pre-flight & Shipping (devx agent ship) ──────
			promStatPanel("Total Preflights", 0, 40, 4, 8,
				`sum(traces_spanmetrics_calls_total{service="devx", span_name="agent_ship_preflight"})`,
				"lastNotNull", "#A5D6FF", "short"),
			promBarGaugePanel("Preflight Success Rate", 4, 40, 8, 8, []promTarget{
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="agent_ship_preflight", status_code!="STATUS_CODE_ERROR"})`, Legend: "passed", RefID: "A"},
				{Query: `sum(traces_spanmetrics_calls_total{service="devx", span_name="agent_ship_preflight", status_code="STATUS_CODE_ERROR"})`, Legend: "failed", RefID: "B"},
			}),
			tablePanel("Recent Agent Ship Preflights", 12, 40, 12, 8,
				`{resource.service.name="devx" && name="agent_ship_preflight"}`),

			// ── Stage 5: Global CLI Health & Versions ─────────────────
			promStatPanel("CLI Error Rate", 0, 48, 4, 8,
				`sum(traces_spanmetrics_calls_total{service="devx", status_code="STATUS_CODE_ERROR"}) / sum(traces_spanmetrics_calls_total{service="devx"}) * 100`,
				"lastNotNull", "#F85149", "percent"),
			promTimeSeriesPanel("Error Rate Over Time", 4, 48, 8, 8,
				`sum(rate(traces_spanmetrics_calls_total{service="devx", status_code="STATUS_CODE_ERROR"}[5m])) / sum(rate(traces_spanmetrics_calls_total{service="devx"}[5m])) * 100`,
				"percent"),
			promBarGaugePanel("Version Distribution", 12, 48, 4, 8, []promTarget{
				{Query: `sum by (service_version) (traces_spanmetrics_calls_total{service="devx"})`, Legend: "{{service_version}}", RefID: "A"},
			}),
			promTimeSeriesPanel("Activity by Version", 16, 48, 8, 8,
				`sum by (service_version) (rate(traces_spanmetrics_calls_total{service="devx"}[5m]))`,
				"cps"),
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
			"uid":  prometheusUID,
		},
		"targets": []map[string]interface{}{
			{
				"datasource": map[string]string{"type": "prometheus", "uid": prometheusUID},
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
			"uid":  prometheusUID,
		},
		"targets": []map[string]interface{}{
			{
				"datasource": map[string]string{"type": "prometheus", "uid": prometheusUID},
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
			"datasource":   map[string]string{"type": "prometheus", "uid": prometheusUID},
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
			"uid":  prometheusUID,
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
			"uid":  tempoUID,
		},
		"targets": []map[string]interface{}{
			{
				"datasource": map[string]string{"type": "tempo", "uid": tempoUID},
				"queryType":  "traceql",
				"query":      query,
				"tableType":  "traces",
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
			"uid":  tempoUID,
		},
		"targets": []map[string]interface{}{
			{
				"datasource": map[string]string{"type": "tempo", "uid": tempoUID},
				"queryType":  "traceql",
				"query":      query,
				"tableType":  "traces",
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
