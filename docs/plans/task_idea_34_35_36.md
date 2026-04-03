# P0 Killer Sprint — Task Tracker

## Phase 1: Research & Foundation
- [x] Read and understand existing codebase structure
- [x] Deep-read `internal/config/config.go`, `internal/provider/`, `internal/devxerr/`, `internal/logs/`
- [x] Understand how `devx db spawn` and `devx mock up` manage container lifecycle

## Phase 2: Idea 36 — Port Conflict Resolution (Lowest risk, enables 34)
- [x] Create `internal/network/ports.go` with `CheckPortAvailable()` and `GetFreePort()`
- [x] Create `internal/network/ports_test.go` with unit tests (5/5 PASS)
- [x] Wire port-checking into `cmd/up.go` database and tunnel boot paths
- [x] Add high-contrast warning output when port shifting occurs

## Phase 3: Idea 35 — Context-Aware Log-Tailing on Crash
- [x] Add crash-log tailing helper to `internal/logs/crashlog.go` (container + host process)
- [x] Wire into `cmd/up.go` error paths for database/mock/service failures
- [x] Add lipgloss styled error box rendering

## Phase 4: Idea 34 — Service Dependency Graphs (Largest change)
- [x] Expand `DevxConfig` struct with `services:`, `depends_on`, and `healthcheck` YAML support
- [x] Implement DAG builder and topological sort in `internal/orchestrator/dag.go`
- [x] Implement healthcheck polling (HTTP and TCP)
- [x] Refactor `cmd/up.go` to use DAG-based boot sequence
- [x] Update `devx.yaml.example` with new `services:` section
- [x] DAG unit tests (7/7 PASS)

## Phase 5: Verification
- [x] `go build ./...` — clean compile, zero errors
- [x] `go test ./...` — all packages pass, zero regressions
- [x] Manual verification (Generated text-based CLI state blocks for docs)
- [x] Create `docs/guide/orchestration.md`
- [x] Update `README.md` marketing language
