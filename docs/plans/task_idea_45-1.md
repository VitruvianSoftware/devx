# Idea 45.1: Bridge Build Telemetry to Local Observability — Task Tracker

## Core Implementation

- [ ] Create `internal/telemetry/export.go` — OTLP HTTP JSON export (fire-and-forget)
- [ ] Create `internal/telemetry/export_test.go` — payload construction, unreachable endpoint, trace ID
- [ ] Modify `internal/telemetry/metrics.go` — add `Attribute` type + variadic attrs, dual-write to OTel
- [ ] Create `internal/telemetry/dashboard.go` — Grafana dashboard JSON + provisioning API
- [ ] Modify `internal/ship/ship.go` — enrich preflight RecordEvent with test/lint/build/branch/stack attrs
- [ ] Modify `cmd/trace_spawn.go` — call ProvisionDashboard after Grafana spawn

## Documentation

- [ ] Update `docs/guide/caching.md` — add OTel observability section
- [ ] Update `docs/guide/trace.md` — add devx build metrics dashboard section
- [ ] Update `FEATURES.md` — add Idea 45.1 entry

## Verification

- [ ] `go vet ./...` clean
- [ ] `go test ./...` all pass
- [ ] Manual: `devx trace spawn grafana` + `devx agent ship` → spans in Grafana
- [ ] Screenshot: capture Grafana dashboard via browser subagent
- [ ] Ship via `devx agent ship`
