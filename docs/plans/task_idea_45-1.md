# Idea 45.1: Bridge Build Telemetry to Local Observability — Task Tracker

## Core Implementation

- [x] Create `internal/telemetry/export.go` — OTLP HTTP JSON export (fire-and-forget)
- [x] Create `internal/telemetry/export_test.go` — payload construction, unreachable endpoint, trace ID
- [x] Modify `internal/telemetry/metrics.go` — add `Attribute` type + variadic attrs, dual-write to OTel
- [x] Create `internal/telemetry/dashboard.go` — Grafana dashboard JSON + provisioning API
- [x] Modify `internal/ship/ship.go` — enrich preflight RecordEvent with test/lint/build/branch/stack attrs
- [x] Modify `cmd/trace_spawn.go` — call ProvisionDashboard after Grafana spawn

## Documentation

- [x] Update `docs/guide/caching.md` — add OTel observability section
- [x] Update `docs/guide/trace.md` — add devx build metrics dashboard section + embedded screenshot
- [x] Update `FEATURES.md` — add Idea 45.1 entry

## Verification

- [x] `go vet ./...` clean
- [x] `go test ./...` all pass
- [x] Manual: `devx trace spawn grafana` + mock telemetry generator → spans in Grafana
- [x] Screenshot: captured Grafana dashboard via browser subagent and mapped into `docs/public/images/grafana-build-metrics.png`
- [x] Ship via `devx agent ship`
