# Task: Idea #31 — Unified OpenAPI & 3rd-Party Mocking

## Phase 1: Config & Schema
- [x] Extend `DevxConfig` in `cmd/up.go` with `mocks []DevxConfigMock`

## Phase 2: Mock Engine
- [ ] Create `internal/mock/server.go` — core lifecycle engine (up, list, restart, rm)

## Phase 3: CLI Commands
- [ ] `cmd/mock.go` — root namespace
- [ ] `cmd/mock_up.go` — `devx mock up`
- [ ] `cmd/mock_list.go` — `devx mock list`
- [ ] `cmd/mock_restart.go` — `devx mock restart`
- [ ] `cmd/mock_rm.go` — `devx mock rm`

## Phase 4: DB Parity
- [ ] `cmd/db_restart.go` — `devx db restart <engine>` (missing parity gap)

## Phase 5: Build & Verification
- [ ] `go build ./...` — confirm clean compile
- [ ] Run `devx mock up` against a real OpenAPI spec
- [ ] Run `devx mock list` to verify daemon visibility
- [ ] Curl the mock endpoint to confirm live response
- [ ] Capture screenshot via browser subagent

## Phase 6: Documentation & Push
- [ ] Create `docs/guide/mocking.md` with embedded screenshot proof
- [ ] Add sidebar entry in `docs/.vitepress/config.mjs`
- [ ] Update `FEATURES.md` and `IDEAS.md`
- [ ] Commit, PR, merge via `/push` workflow
- [ ] Verify CI green
