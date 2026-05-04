# Task: Idea 59 & 60 — AI Failure Recovery + DB Ask

## Idea 59: Intelligent Failure Recovery
- [x] `internal/ai/diagnose.go` — Rule-based + LLM diagnosis engine
- [x] `internal/ai/diagnose_test.go` — Unit tests (8 tests, all pass)
- [x] `internal/devxerr/error.go` — Add new exit codes (90-93)
- [x] `cmd/root.go` — Hook diagnosis into `Execute()`

## Idea 60: Natural Language DB Queries
- [x] `internal/database/query.go` — Canned queries + read-only executor + table renderer
- [x] `internal/database/query_test.go` — Unit tests (7 tests, all pass)
- [x] `cmd/db_ask.go` — Cobra command with canned + NL paths

## Verification
- [x] Build passes (`go build`)
- [x] Lint passes (`golangci-lint run` — 0 issues)
- [x] All 32 tests pass (ai: 14, database: 18)
- [x] Help text renders correctly

## Documentation
- [x] `FEATURES.md` — Added Ideas 59 & 60 as shipped
- [x] `IDEAS.md` — Removed shipped ideas from active list
