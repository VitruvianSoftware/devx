# Idea 44: Unified Multirepo Orchestration — Tasks

## Phase 1: Config Engine (Foundation)
- [x] Create `cmd/config.go` — unified `DevxConfig` structs + `resolveConfig()` helper
  - [x] Move/expand all `DevxConfig*` types from `up.go`
  - [x] Add `Seed`/`Pull` to `DevxConfigDatabase`
  - [x] Add `Dir string` to `DevxConfigService` and `DevxConfigDatabase`
  - [x] Add `DevxConfigInclude` struct
  - [x] Add `Include []DevxConfigInclude` to `DevxConfig`
  - [x] Implement `resolveIncludes()` (recursive, depth-limit 5, dedup by abs path)
  - [x] Implement `resolveConfig(path, profile)` public API

## Phase 2: Command Refactors (Use shared resolver)
- [x] `cmd/up.go` — remove `DevxConfig*` types, use `resolveConfig()`, pass `Dir` to DAG nodes
- [x] `cmd/sync_up.go` — replace manual YAML parsing with `resolveConfig()`
- [x] `cmd/db_seed.go` — use `resolveConfig()`, add `cmd.Dir = db.Dir` to seed exec
- [x] `cmd/test_ui.go` — use `resolveConfig()`
- [x] `cmd/map.go` — use `resolveConfig()`
- [x] `cmd/config_pull.go` — use `resolveConfig()` for `env:` block
- [x] `cmd/config_push.go` — use `resolveConfig()` for `env:` block
- [x] `cmd/config_validate.go` — use `resolveConfig()`

## Phase 3: Orchestrator Working Directory
- [x] `internal/orchestrator/dag.go` — add `Dir string` to `Node`
- [x] `internal/orchestrator/dag.go` — set `cmd.Dir = n.Dir` in `startHostProcess`

## Phase 4: devx.yaml.example
- [x] Add `include` example block

## Phase 5: Build & Verification
- [x] `go build ./...`
- [x] `go vet ./...`
- [x] `go test ./...`
- [x] Manual: `devx up --dry-run` with include pointing to temp project
- [x] Edge case: duplicate service name → fail-fast error
- [x] Edge case: missing include path → clear error
- [x] Edge case: circular include (depth cap)
- [x] Edge case: `devx sync up` sees included sync blocks

## Phase 6: Documentation
- [x] Create `docs/guide/multirepo.md`
- [x] Update `docs/.vitepress/config.mjs` sidebar
- [x] Update `README.md` feature list
- [x] Move Idea 44: `IDEAS.md` → `FEATURES.md`
- [x] `npm run docs:build`
