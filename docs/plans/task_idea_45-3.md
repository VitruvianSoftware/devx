# Task List: Idea 45.3 (Granular Test Telemetry)

## Phase 1: Dashboard JSON Resolution (Workstream A)
- [x] Fix `statPanel` queries (wrap in `count_over_time`/`quantile_over_time` and swap to generic `stat` types or configure correctly)
- [x] Refactor pie charts to bar gauge panels with split query targets (Pass vs Fail)
- [x] Add "Test Details" `table` panel to dashboard
- [x] Apply `dashboardJSON()` updates to `internal/telemetry/dashboard.go`

## Phase 2: devx run --name Propagation (Workstream C)
- [x] Update `RecordEvent("devx_run", ...)` in `cmd/run.go` to include `telemetry.Attr("devx.name", name)`

## Phase 3: Go Test Interceptor (Workstream B)
- [x] Create `internal/telemetry/test_reporter.go`
- [x] Implement `IsGoTestCmd(args)` and `InjectJSONFlag(args)`
- [x] Implement `bufio.Scanner` stdout proxy and JSON unmarshaling in `RunGoTestWithTelemetry()`
- [x] Reconstruct console output (`✓ PASS`, `✗ FAIL`, `○ SKIP`) and passthrough standard output
- [x] Wire up `telemetry.ExportSpan("go_test", ...)` for individual tests

## Phase 4: Integration
- [x] Modify `cmd/run.go` to intercept `go test` with `RunGoTestWithTelemetry`
- [x] Modify `internal/ship/ship.go` to intercept `go test` in pipeline runner
- [ ] Update documentation (`FEATURES.md`, `docs/guide/pipeline.md`, `docs/guide/trace.md`)

## Phase 5: Verification
- [x] Verify `devx trace spawn grafana` generates correct JSON
- [x] Run `devx run --name test -- go test ./...` and verify test spans in Grafana
- [x] Review UI data presentation using the browser subagent
