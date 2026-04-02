# Idea 29: Shift-Left Distributed Observability

This plan implements `devx trace`, a zero-config local distributed tracing stack. It allows developers to instantly spin up an OpenTelemetry-compliant backend to visualize spans natively during local development, choosing between extreme lightweight execution or full-stack Grafana familiarity.

## User Review Required

> [!NOTE]
> **Addressing Feedback: Dual-Engine Support**
> Per your request, `devx trace spawn` will support two engines to give developers familiarity and choice:
> 1. `devx trace spawn jaeger` (Default): Spins up the lightweight `jaegertracing/all-in-one:latest` (UI on 16686, OTLP on 4317/4318).
> 2. `devx trace spawn grafana`: Spins up the official `grafana/otel-lgtm:latest` image. This is a brilliant single-container distribution by Grafana that embeds a native **OTEL Collector**, **Tempo** (tracing), **Loki** (logs), **Prometheus** (metrics), and **Grafana** (UI on 3000). It fully satisfies the "otel+grafana" familiarity requirement without the orchestrational nightmare of spinning up 5 separate containers locally. Is this `otel-lgtm` distribution acceptable for the Grafana tier?

## Proposed Changes

---

### `cmd/` (CLI Commands)

#### [NEW] `cmd/trace.go`
- Registers the root `devx trace` command category.

#### [NEW] `cmd/trace_spawn.go`
- Implements `devx trace spawn [engine]`.
- **Engine: `jaeger`** (Default)
  - Starts `docker.io/jaegertracing/all-in-one:latest` with the label `devx-telemetry=jaeger`.
  - Binds ports: `4317` (OTLP gRPC), `4318` (OTLP HTTP), `16686` (Web UI).
  - Emits terminal instructions pointing to `http://localhost:16686`.
- **Engine: `grafana`**
  - Starts `docker.io/grafana/otel-lgtm:latest` with the label `devx-telemetry=grafana`.
  - Binds ports: `4317`, `4318`, `3000` (Grafana UI), `4319` (Prometheus), `3200` (Tempo).
  - Emits terminal instructions pointing to `http://localhost:3000` (default creds admin:admin).

#### [NEW] `cmd/trace_list.go` & `cmd/trace_rm.go`
- Clean `list` and `rm` commands for the telemetry containers, matching the `devx cloud` and `devx db` patterns.

---

### `internal/telemetry/` (Core Logic)

#### [NEW] `internal/telemetry/otel.go`
- Handles discovery: `DiscoverOTEL(runtime string) (map[string]string, bool)`
- Scans `docker ps` / `podman ps` for any container matching the label key `devx-telemetry`.
- Constructs the environment variable map to force OTEL SDKs to beam data locally:
  ```env
  OTEL_EXPORTER_OTLP_ENDPOINT=http://host.containers.internal:4318
  OTEL_TRACES_EXPORTER=otlp
  OTEL_METRICS_EXPORTER=none # (or otlp if they are running the grafana engine!)
  ```

---

### `cmd/shell.go` (Integration)

#### [MODIFY] `cmd/shell.go`
- Invokes `telemetry.DiscoverOTEL(runtime)` during devcontainer launch.
- If a local trace backend is found, appends the OTEL environment variables to the dev container `docker run` args.
- Informs the developer: `🔍 Local telemetry detected ([engine]). Injected OTEL_EXPORTER_OTLP_ENDPOINT.`

## Open Questions

1. Do you want to expose legacy endpoints like Zipkin (`9411`) or older thrift ports for backwards compatibility, or strictly push developers towards modern OpenTelemetry (OTLP) endpoints?
2. Local telemetry traces will remain strictly ephemeral (stored in container memory/tmpfs) to prevent disk bloat over time. When a developer runs `devx trace rm`, all their traces will be wiped. Is this clean-slate approach preferred?

## Verification Plan

### Automated Tests
- The proposed `devx shell` injection logic is tested natively, ensuring `OTEL_EXPORTER_OTLP_ENDPOINT` is added when `telemetry.DiscoverOTEL()` discovers a telemetry engine correctly.

### Manual Verification
- Execute `devx trace spawn grafana`.
- Open `http://localhost:3000` to verify the Grafana UI and OTEL Collector is running.
- Execute `devx trace spawn jaeger`.
- Open `http://localhost:16686` to verify the Jaeger UI works.
- Execute `devx shell` and run `echo $OTEL_EXPORTER_OTLP_ENDPOINT` to verify container-to-host connectivity string is properly assigned.
