# Idea 45.2: Declarative Pipeline Stages — Task Tracker

## Core Implementation

- [x] Add `PipelineStage`, `PipelineConfig`, `CustomAction` types to `cmd/devxconfig.go`
- [x] Pipeline types parse from devx.yaml (including hooks scaffolded for 45.3)
- [x] Enhance `cmd/run.go` — universal telemetry wrapper with --dry-run, exit code, log routing
- [x] Refactor `internal/ship/ship.go` — `RunPreFlight` accepts optional `*PipelineConfig`
- [x] Update `cmd/agent_ship.go` — loads `devx.yaml` pipeline config before preflight
- [x] Add `convertPipeline`/`convertStage` bridge functions
- [x] **FIX**: Add `telemetry.Flush()` barrier to ensure fast-executing `devx run` commands do not slice the OTLP HTTP post goroutine mid-flight on `os.Exit`.

## Documentation & Dogfooding

- [x] Create `devx.yaml` in project root (dogfooding our own pipeline)
- [x] Add "Familiarity-First" design principle to `docs/guide/introduction.md`
- [x] Create `docs/guide/pipeline.md` — dedicated pipeline documentation
- [x] Update `devx.yaml.example` with pipeline + hooks + customActions examples
- [x] Update `FEATURES.md` with Idea 45.2 entry
- [x] Update `docs/.vitepress/config.mjs` sidebar with pipeline guide
- [x] **FIX**: Update auto-provisioned "devx Build Metrics" Grafana dashboard to explicitly query and display `devx_run` spans alongside traditional CI spans.

## Verification

- [x] `go vet ./...` clean
- [x] `go test ./...` all pass
- [x] Dogfood: `devx run -- go vet ./...` — telemetry recorded
- [x] Dogfood: `devx run -- go test ./...` — telemetry recorded
- [x] Dogfood: `devx run --dry-run -- go test ./...` — prints intent without executing
- [x] Verified `devx_run` events in ~/.devx/metrics.json
- [x] Verified `devx_run` spans successfully ingest into Tempo backend and appear in the devx Build Metrics dashboard.
