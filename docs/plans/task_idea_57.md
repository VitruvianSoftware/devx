# Task: Idea 57 — AI-Driven Synthetic Data Generation

## Phase 1: Engine (internal packages)
- [x] `internal/devxerr/error.go` — Add exit codes 86-89
- [x] `internal/ai/completion.go` — `AIProvider`, `DiscoverAIProvider()`, `GenerateCompletion()`
- [x] `internal/ai/completion_test.go` — Mock HTTP server tests
- [x] `internal/database/synthesizer.go` — `ExtractSchema()`, `SanitizeLLMSQL()`, `PipeSQL()`
- [x] `internal/database/synthesizer_test.go` — Unit tests

## Phase 2: CLI
- [x] `cmd/db_synthesize.go` — Full command implementation

## Phase 3: Verification
- [x] `go vet ./...`
- [x] `golangci-lint run ./...`
- [x] `go test -race ./...`
- [x] `mage licensecheck`

## Phase 4: Documentation
- [x] `docs/guide/databases.md` — Add AI Synthetic Data section
- [x] `cmd/doctor.go` — Add feature readiness entry
- [x] `README.md` — Add db synthesize mention
- [x] `.agents/skills/devx/SKILL.md` — Add command + error codes
- [x] `.agents/skills/platform-engineer/SKILL.md` — Add command
- [x] `.github/skills/devx/SKILL.md` — Mirror
- [x] `.github/skills/platform-engineer/SKILL.md` — Mirror
- [x] `CONTRIBUTING.md` — Add `internal/ai/`
- [x] `FEATURES.md` — Migrate Idea 57 from IDEAS.md
- [x] `IDEAS.md` — Remove Idea 57
