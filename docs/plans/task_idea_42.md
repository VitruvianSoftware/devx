# Task: devx ci run (Idea 42)

## Phase 1: Parsing Engine
- [x] `internal/ci/parser.go` — Workflow/Job/Step structs, matrix expansion, DAG resolution
- [x] `internal/ci/template.go` — `${{ }}` expression substitution engine
- [x] `internal/ci/parser_test.go` — Parse real CI workflow, test matrix + DAG + conditionals

## Phase 2: Execution Engine
- [x] `internal/ci/executor.go` — Container lifecycle, step execution, services provisioning
- [x] `internal/ci/writer.go` — Thread-safe prefixed line streaming for parallel output

## Phase 3: CLI Commands
- [x] `cmd/ci.go` — Root `devx ci` command
- [x] `cmd/ci_run.go` — `devx ci run` with flags, workflow discovery, dry-run

## Phase 4: Documentation & Ship
- [x] `docs/guide/ci.md` — Usage, limitations, comparison
- [x] Update `docs/.vitepress/config.mjs` sidebar
- [x] Update `FEATURES.md` and `README.md`
- [x] Build, test, ship, release → v0.27.0
