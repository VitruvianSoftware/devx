# Idea 29: Shift-Left Distributed Observability

This plan implements `devx trace`, a zero-config local distributed tracing stack. It allows developers to instantly spin up an OpenTelemetry-compliant backend to visualize spans natively during local development, choosing between extreme lightweight execution (`jaeger`) or full-stack Grafana familiarity (`grafana`).

## User Review Required

> [!TIP]
> **Plan finalized based on your latest feedback:**
> 1. **Modern OTLP Only:** Legacy ports (Zipkin/Thrift) have been completely stripped from the bindings. We will enforce bleeding-edge standard OTLP (`4317`/`4318`) across the board.
> 2. **`--persist` Flag Added:** A `--persist` flag will be added to `devx trace spawn`. When active, trace and metrics data will be bind-mounted into `~/.devx/telemetry/` on the Mac host, surviving `devx trace rm` events.
> 3. **Trace Simulation Verification:** I have added a concrete verification step using a raw `curl` payload to fire a synthetic trace from *inside* `devx shell` and verify it lands correctly in the UIs.
>
> If this plan looks perfectly aligned with your vision, give me the go-ahead and I will begin the implementation!

## Proposed Changes

---

### `cmd/` (CLI Commands)

#### [NEW] `cmd/trace.go`
- Registers the root `devx trace` command category.

#### [NEW] `cmd/trace_spawn.go`
- Implements `devx trace spawn [engine] [--persist]`.
- **Engine: `jaeger`** (Default)
  - Starts `docker.io/jaegertracing/all-in-one:latest`.
  - Binds ports: `4317` (OTLP gRPC), `4318` (OTLP HTTP), `16686` (Web UI).
  - Handles `--persist`: Maps a volume to `/badger` and sets `SPAN_STORAGE_TYPE=badger`.
- **Engine: `grafana`**
  - Starts `docker.io/grafana/otel-lgtm:latest`.
  - Binds ports: `4317`, `4318`, `3000` (Grafana UI), `4319` (Prometheus), `3200` (Tempo).
  - Handles `--persist`: Maps a host volume to `/data/` inside the container.
- Emits terminal instructions pointing to the respective UIs.

#### [NEW] `cmd/trace_list.go` & `cmd/trace_rm.go`
- Clean `list` and `rm` commands for the telemetry containers, matching the `devx cloud` patterns.

---

### `internal/telemetry/` (Core Logic)

#### [NEW] `internal/telemetry/otel.go`
- Handles discovery: `DiscoverOTEL(runtime string) (map[string]string, bool)`
- Scans `docker ps` / `podman ps` for any container matching the label `devx-telemetry`.
- Constructs the environment variable map:
  ```env
  OTEL_EXPORTER_OTLP_ENDPOINT=http://host.containers.internal:4318
  OTEL_TRACES_EXPORTER=otlp
  OTEL_METRICS_EXPORTER=none # prevents sdk spam on jaeger; grafana engine can accept metrics
  ```

---

### `cmd/shell.go` (Integration)

#### [MODIFY] `cmd/shell.go`
- Invokes `telemetry.DiscoverOTEL(runtime)` during launch.
- Appends the OTEL variables to the `docker run` args dynamically.

## Open Questions
- None. The plan has been refined based on all prior feedback and is ready for execution.

## Verification Plan

### Automated Tests
- Verify that `OTEL_EXPORTER_OTLP_ENDPOINT` is injected into `devx shell` arguments conditionally.

### Manual End-to-End Simulation
1. ** Jaeger Flow:**
   - Execute `devx trace spawn jaeger`
   - Enter `devx shell`
   - Execute an explicit synthetic raw OTLP payload matching the environment variable:
     ```bash
     curl -i -X POST "$OTEL_EXPORTER_OTLP_ENDPOINT/v1/traces" \
     -H "Content-Type: application/json" -d '{
     "resourceSpans": [{"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "my-devx-test"}}]}, "scopeSpans": [{"spans": [{"traceId": "12345678901234567890123456789012", "spanId": "1234567890123456", "name": "Synthetic Test Span", "kind": 1, "startTimeUnixNano": "1700000000000000000", "endTimeUnixNano": "1700000005000000000"}]}]}]}'
     ```
   - Open `http://localhost:16686` and assert the trace appears.
   
2. **Grafana Flow:**
   - Terminate Jaeger: `devx trace rm jaeger`
   - Start Grafana persist: `devx trace spawn grafana --persist`
   - Enter `devx shell`
   - Re-run the raw `curl` synthetic trace command.
   - Open `http://localhost:3000` -> Explore -> Tempo -> assert trace `12345678901234567890123456789012` is rendered properly.
