# Idea 45: Predictive Background Pre-Building — Task Tracker

## Phase 1: Core Principles & Telemetry Foundation

- [x] Add "Future-Proofing for Growth" design principle to `docs/guide/introduction.md`
- [x] Create `internal/telemetry/metrics.go`
  - [x] `RecordEvent()` with flock-based file locking
  - [x] `NudgeIfSlow()` with threshold + stderr output
  - [x] FIFO rotation at 1000 entries
- [x] Create `internal/telemetry/metrics_test.go`
  - [x] Test RecordEvent writes
  - [x] Test FIFO rotation
  - [x] Test NudgeIfSlow threshold behavior
  - [x] Test corrupted file recovery
- [x] Instrument `internal/ship/ship.go` — wrap build step with timing
- [x] Instrument `cmd/up.go` — wrap `dag.Execute()` with timing

## Phase 2: `devx stats` Command

- [x] Create `cmd/stats.go` — percentile display, `--json`, `--clear`
- [x] Create `cmd/stats_test.go` — percentile calculation, empty data, JSON output

## Documentation & Project Records

- [x] Update `devx.yaml.example` — add commented `predictive_build: true`
- [x] Create `docs/guide/caching.md` — feature guide
- [x] Update `docs/.vitepress/config.mjs` — sidebar entry
- [x] Update `README.md` — feature bullet
- [x] Migrate Idea 45 from `IDEAS.md` to `FEATURES.md`

## Verification

- [x] `go vet ./...` clean
- [x] `go test ./...` all pass
- [x] Ship via `devx agent ship` — PR #120 (feat), PR #121 (lint fix)
- [x] CI green on main
