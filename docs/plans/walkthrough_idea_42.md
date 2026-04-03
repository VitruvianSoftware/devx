# Walkthrough: `devx ci run` (Idea 42) — v0.27.0

## What Was Built

A local GitHub Actions CI pipeline emulator that parses `.github/workflows/*.yml` and executes `run:` shell blocks inside isolated Podman/Docker containers with ~80% parity.

## Files Created

| File | Purpose |
|------|---------|
| [parser.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/parser.go) | Workflow/Job/Step structs, matrix cartesian expansion, DAG resolution via Kahn's algorithm |
| [template.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/template.go) | `${{ }}` expression substitution (env, secrets, matrix, github/runner stubs) |
| [executor.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/executor.go) | Container lifecycle (one per job), step execution, parallel tier orchestration |
| [writer.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/writer.go) | Thread-safe prefixed line streaming for parallel output |
| [parser_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ci/parser_test.go) | 8 tests: real workflow parsing, matrix expansion, DAG, templates, conditionals |
| [ci.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/ci.go) | Root `devx ci` command |
| [ci_run.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/ci_run.go) | `devx ci run` with workflow discovery, huh picker, flags, dry-run |
| [ci.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/ci.md) | Official docs with explicit limitation section and act comparison |

## Key Design Decisions

1. **`uses:` is intentionally unsupported** — skipped with a visible `⚠️  SKIPPED` warning. Documented in CLI help, code comments, and official docs.
2. **One container per job** (not per step) — preserves filesystem state between steps, matching GitHub's behavior.
3. **Docker Compose-style prefixed output** instead of Bubble Tea TUI — preserves `--json`, `--non-interactive`, and pipe compatibility.
4. **Parallel matrix via goroutines** — faithfully mimics GitHub's parallel matrix execution.
5. **Fail-open conditionals** — complex `if:` expressions default to `true` with a warning rather than silently skipping steps.
6. **Template engine** scoped to `env`/`secrets`/`matrix` with explicit stubs for `github.*`/`runner.*` contexts.

## What Was Tested

- 8 unit tests passing against the real `devx` CI workflow
- `go build ./... && go vet ./... && go test ./...` — all green
- CI pipeline green on GitHub Actions
- Release v0.27.0 shipped with all 4 platform binaries verified

## Files Modified

- [config.mjs](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/.vitepress/config.mjs) — Added sidebar link
- [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md) — Migrated Idea 42 from IDEAS.md
- [IDEAS.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/IDEAS.md) — Removed completed Idea 42
- [README.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/README.md) — Added CI emulation bullet
