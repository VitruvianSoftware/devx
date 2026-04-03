# Idea 40: `devx agent ship` — Task Tracker

## Phase 1: Core Implementation
- [x] Examine existing `cmd/agent.go` and `cmd/init.go` patterns
- [x] Create `internal/ship/` package (pre-flight, push, CI polling)
- [x] Create `cmd/agent_ship.go` command
- [x] Create `cmd/hook.go` for `devx hook pre-push`
- [x] Update `cmd/init.go` — N/A (hook installed via `devx agent ship --install-hook`)

## Phase 2: Doctor Expansion
- [x] Add hook diagnosis to `devx doctor`
- [x] Add `devx agent ship` feature readiness to `devx doctor`

## Phase 3: Documentation & Skills
- [x] Update `docs/guide/ai-agents.md` with `devx agent ship` documentation
- [x] Update all agent SKILL.md files (`.agent`, `.claude`, `.cursor`, `.github`, templates)

## Phase 4: Verification — Dogfooding
- [x] Ship this feature using `devx agent ship` itself (PR #105)
- [x] Verify pre-push hook blocks raw `git push` ✅ (Exit Code 1)
- [x] Verify CI pipeline status is reported from tool stdout ✅ (run 23931209099 — GREEN)
