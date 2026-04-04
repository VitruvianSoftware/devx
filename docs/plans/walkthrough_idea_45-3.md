# Walkthrough: Idea 45.3 — Granular Test Telemetry & Dashboard Resolution

## Root Cause Analysis

The dashboard was completely broken due to **two independent failures**:

### 1. `traceqlmetrics` Query Type Not Supported (Grafana 12.4.2)
The Grafana Tempo plugin in v12.4.2 does **not** support `queryType: "traceqlmetrics"`. Every stat, time series, and bar gauge panel was using this query type, causing:

```
error="client: failed to query data: unsupported query type: 'traceqlmetrics' for query with refID 'A'"
```

**Fix**: Switched all aggregate metric panels to use **Prometheus** as the datasource. Tempo's `span_metrics` generator already remote-writes to Prometheus (`traces_spanmetrics_calls_total`, `traces_spanmetrics_latency_*`), so we query these counters/histograms via PromQL instead.

### 2. Zero-Duration Test Spans
Fast Go tests report `Elapsed: 0.00` in the `test2json` stream. Our `ExportSpan("go_test", ...)` was called with `duration = 0`, producing spans where `startTimeUnixNano == endTimeUnixNano`.

**Fix**: Track test start times via the `"run"` action, compute wall-clock duration on terminal events, and enforce a 1µs minimum floor.

### 3. Table Missing Test Attributes
The `tablePanel` TraceQL query was `{name="go_test"}` which only returned standard OTLP fields. Span attributes like `devx.test.name` weren't surfaced.

**Fix**: Added `| select(span.devx.test.name, span.devx.test.status, span.devx.test.package)` to the TraceQL query, which instructs Tempo to project those attributes as columns.

---

## Changes Made

### [dashboard.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/telemetry/dashboard.go) — Complete Rewrite
- Replaced `traceqlmetrics` → **Prometheus PromQL** for all stat/timeseries/bargauge panels
- Panel queries now target `traces_spanmetrics_calls_total` and `traces_spanmetrics_latency_*`
- Added `promStatPanel`, `promTimeSeriesPanel`, `promBarGaugePanel` builders
- Added `tablePanelWithSelect` for attribute-projected Tempo queries
- New panels: "Total Tests" (129), "Test Execution Rate", "Test Activity"

### [test_reporter.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/telemetry/test_reporter.go) — Bug Fixes
- Track `"run"` events to compute wall-clock duration for fast tests
- 1µs minimum floor for span width
- `sync.WaitGroup` to prevent race between goroutine and `cmd.Wait()`
- Nil-safe `stdout`/`stderr` writers

### [run.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/run.go) — Integration
- Go test interception via `telemetry.IsGoTestCmd` → `RunGoTestWithTelemetry`
- `devx.name` attribute propagation to OTLP spans

### [ship.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ship/ship.go) — Integration
- `runCmd()` intercepts Go test commands in both explicit and auto-detected pipelines

### Documentation
- `FEATURES.md` — Added 45.3 entry
- `docs/guide/pipeline.md` — Added "Granular Test Telemetry" section
- `docs/guide/trace.md` — Updated panel descriptions and attribute docs

---

## Verification Results

### Dashboard Screenshot (all panels rendering)

![devx Build Metrics Dashboard — All Panels Active](/images/grafana-build-metrics.png)

| Panel | Status | Value |
|-------|--------|-------|
| Total Builds | ✅ | 1 |
| Total Preflights | "No data" | Expected — no `agent ship` runs |
| Avg Build Time | ✅ | 2.49 s |
| Total Tests | ✅ | 129 |
| Build Duration Over Time | ✅ | Line chart rendered |
| Recent Commands | ✅ | 1 row: `devx_run`, 2.49s |
| Build & Run Activity | ✅ | Bar: 1 |
| Test Activity | ✅ | Bar: 129 |
| Test Execution Rate | ✅ | Rate spike rendered |
| Test Details | ✅ | Test names, packages, statuses visible |

### Browser Recording

![Dashboard Verification Recording](/images/grafana-build-metrics-recording.webp)
