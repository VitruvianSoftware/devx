# Idea 45.1: Bridge Build Telemetry to Local Observability

## Background

We just shipped Idea 45 (Phase 1-2): a local-only metrics engine that records build/startup durations to `~/.devx/metrics.json`. Separately, Idea 29 (`devx trace spawn`) provisions local Jaeger or Grafana LGTM backends that accept OpenTelemetry data on `localhost:4318`.

**The pivot:** Instead of keeping these two systems isolated, make `devx` itself emit OTel spans and metrics to the local backend whenever one is running. This creates a beautiful dogfooding loop: developers using `devx` to build `devx` will see their own build times, test results, and lint outcomes visualized in Grafana/Jaeger — demonstrating two shipped features working together.

## 1. Design Decisions

### No New Go Dependencies
The OTel Go SDK is a heavy import tree (~15 packages). Instead, we'll use the **OTLP HTTP JSON API** directly. The OTLP/HTTP endpoint at `localhost:4318` accepts JSON payloads via simple `net/http` POST — no SDK required. This keeps the binary lean and is perfectly aligned with our "Zero-Install Philosophy".

### Dual-Write Pattern
`RecordEvent()` already writes to `~/.devx/metrics.json`. We'll add an opportunistic secondary write: if `localhost:4318` is reachable, also POST the event as an OTLP span. If the backend isn't running, the POST silently fails (fire-and-forget, consistent with the existing design).

### Rich Span Attributes
Each span will carry structured attributes beyond just duration:
- `devx.event` — event name (e.g., `agent_ship_build`)
- `devx.stack` — detected stack (Go, Node/JS/TS, Rust, Python)
- `devx.test.pass`, `devx.lint.pass`, `devx.build.pass` — boolean outcomes
- `devx.test.skipped`, `devx.lint.skipped`, `devx.build.skipped`
- `devx.branch` — current git branch
- `devx.project` — project directory basename

### Grafana Dashboard Provisioning
For users running `devx trace spawn grafana`, we'll auto-provision a Grafana dashboard JSON via the Grafana HTTP API (`localhost:3000`). The dashboard will include panels for:
- Build duration over time (line chart)
- Test/Lint/Build pass rate (pie chart)
- P50/P90/P99 build latency (stat panels)
- Recent build history (table)

## 2. Proposed Changes

---

### [NEW] `internal/telemetry/export.go`

**Functions:**

```go
// ExportSpan posts a single span to a local OTLP/HTTP endpoint.
// Fire-and-forget: silently no-ops if the backend is unreachable or returns an error.
func ExportSpan(span OTLPSpan)

// OTLPSpan is a lightweight span struct for OTLP JSON export.
type OTLPSpan struct {
    Name           string
    StartTimeNanos uint64
    EndTimeNanos   uint64
    Attributes     map[string]interface{}
}
```

Implementation Details:
1. **ID Generation**: Uses `crypto/rand` to generate a 16-byte hex `traceId` and an 8-byte hex `spanId` per the OTLP spec.
2. **Resource Definition**: Wraps the span in a `ResourceSpans` block with a `service.name = devx` attribute.
3. **HTTP Delivery**: Constructs the OTLP JSON payload per the [OTLP/HTTP spec](https://opentelemetry.io/docs/specs/otlp/#otlphttp), POSTs to `http://localhost:4318/v1/traces` with a 2-second timeout. No retries.

### [MODIFY] `internal/telemetry/metrics.go`

- `RecordEvent()` gains an optional variadic `...Attribute` parameter (key-value pairs) to carry rich metadata.
- After writing to JSON, calls `ExportSpan()` with the same data.

### [MODIFY] `internal/ship/ship.go`

- `RunPreFlight()` now records enriched attributes:
  ```go
  telemetry.RecordEvent("agent_ship_preflight", totalDur,
      telemetry.Attr("devx.stack", stack.Name),
      telemetry.Attr("devx.test.pass", result.TestPass),
      telemetry.Attr("devx.lint.pass", result.LintPass),
      telemetry.Attr("devx.build.pass", result.BuildPass),
      telemetry.Attr("devx.branch", CurrentBranch(dir)),
      telemetry.Attr("devx.project", filepath.Base(dir)),
  )
  ```
- The existing build-only `RecordEvent("agent_ship_build", ...)` stays for backward compatibility; the new preflight span covers the full pipeline.

### [NEW] `internal/telemetry/dashboard.go`

- Contains the Grafana dashboard JSON as an embedded Go string constant.
- Exposes `ProvisionDashboard()` which sends a `POST` request to `http://localhost:3000/api/dashboards/db`.
- **Authentication**: Uses HTTP Basic Auth (`admin`:`admin` - the default for the LGTM stack image). If authentication fails, it degrades gracefully (fire-and-forget) to handle cases where the user modified the default password.
- Called automatically from `cmd/trace_spawn.go` when the engine is `grafana`.

### [MODIFY] `cmd/trace_spawn.go`

- After the Grafana container starts and is healthy, call `telemetry.ProvisionDashboard()` to auto-install the "devx Build Metrics" dashboard.
- Print the dashboard URL in the post-spawn info block.

### [NEW] `internal/telemetry/export_test.go`

- Test OTLP JSON payload construction (verify spec compliance).
- Test fire-and-forget behavior (no panic on unreachable endpoint).
- Test attribute serialization.

## 3. Dashboard Panels (Grafana)

| Panel | Type | Data Source | Query |
|-------|------|-------------|-------|
| Build Duration Over Time | Time series | Tempo | Spans where `devx.event = agent_ship_build` |
| Pre-Flight Pass Rate | Pie chart | Tempo | Count of `devx.build.pass = true` vs `false` |
| P50/P90/P99 Build Latency | Stat | Tempo | Duration percentiles from build spans |
| Recent Builds | Table | Tempo | Last 20 spans with stack, branch, pass/fail, duration |
| Test Results | Bar gauge | Tempo | Pass/Skip/Fail counts for test, lint, build |

## 4. Global Flags Compliance

| Flag | Behavior |
|------|----------|
| `--json` | OTel export still fires (it's internal, not user-facing output) |
| `--dry-run` | Spans still emitted (recording a dry-run is still useful telemetry) |
| `--runtime` | Used to detect container runtime for `DiscoverOTEL()` |

## 5. Edge Cases

| Scenario | Handling |
|----------|----------|
| No telemetry backend running | `ExportSpan` silently no-ops (2s timeout on POST) |
| Jaeger running (not Grafana) | Spans still export to Jaeger; no dashboard provisioned |
| Grafana API unreachable during dashboard provisioning | Warning printed, not fatal |
| Dashboard already exists | Grafana API `overwrite: true` flag updates it in-place |
| Rapid successive builds | Each build is a separate span; no deduplication needed |

## 6. Documentation Updates

| Document | Change |
|----------|--------|
| `docs/guide/caching.md` | Add section on OTel observability integration |
| `docs/guide/trace.md` | Add note about devx build metrics dashboard |
| `FEATURES.md` | Add Idea 45.1 entry |

## 7. Verification & Documentation Screenshots

### Automated Tests
- `go vet ./...` clean
- `go test ./...` all pass  
- `internal/telemetry/export_test.go`: OTLP payload construction, reachable/unreachable endpoint safety, trace ID generation.

### Manual Verification & Screenshot Capture
1. Spawn the backend: `devx trace spawn grafana` 
2. Trigger telemetry: Run `devx agent ship` on the devx repo to ensure pre-flight spans are exported.
3. Verify Dashboard: Open `http://localhost:3000` → navigate to "devx Build Metrics" dashboard → verify panels render.
4. **Documentation Asset Generation:** I will use the **browser_subagent** to navigate to `localhost:3000`, log in as `admin`/`admin`, navigate to the newly provisioned "devx Build Metrics", and capture a high-quality screenshot (`grafana_build_metrics.png`).
5. Embed the screenshot into the official `docs/guide/trace.md`.

> [!IMPORTANT]
> **Ready for approval.** This is a quick, high-impact pivot connecting two existing features. Zero new Go dependencies — just `net/http` POST to the OTLP endpoint. Should I proceed?
