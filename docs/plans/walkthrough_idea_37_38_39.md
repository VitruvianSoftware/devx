# P1 Polish Pass — Walkthrough

**PR:** [#97](https://github.com/VitruvianSoftware/devx/pull/97)
**Branch:** `feat/p1-polish-pass`

---

## Changes Made

### Idea 37: Environment Profiles (`--profile`)

| File | Change |
|------|--------|
| [up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go) | Added `DevxConfigProfile` struct, `Profiles` map field on `DevxConfig`, `--profile` CLI flag, and `mergeProfile()` with additive/merge semantics |
| [profile_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/profile_test.go) | 6 unit tests covering service add/merge, database add/merge, tunnel merge with field preservation, and empty profile no-op |
| [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example) | Added comprehensive `profiles:` section with `backend-only` and `staging` examples, fully documented |

**Key Design Decision:** Uses Docker Compose override semantics — matching entries are field-merged (profile wins), new entries are appended, unmentioned base entries are preserved.

---

### Idea 38: Native Secrets Redaction

| File | Change |
|------|--------|
| [redactor.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/logs/redactor.go) | `SecretRedactor` with longest-first sorted replacement, non-sensitive key filtering, and `[REDACTED]` marker |
| [redactor_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/logs/redactor_test.go) | 7 tests: basic replacement, longest-first ordering, short value skip, non-sensitive key skip, multiple occurrences, empty input, no secrets |
| [streamer.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/logs/streamer.go) | Added `Redactor` field on `Streamer`, applied in all 3 emission points (container stdout/stderr, host file tailing) |
| [tui.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/logs/tui.go) | Added `InitialModelWithRedactor()` constructor |
| [logs.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/logs.go) | Wired `NewSecretRedactor()` into both JSON and TUI paths |

---

### Idea 39: Visual Architecture Map (`devx map`)

| File | Change |
|------|--------|
| [map.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/map.go) | New `devx map` command with Mermaid flowchart generation, styled class definitions, and `--output` file support |
| [map_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/map_test.go) | 4 tests: full topology validation, empty config, container runtime labels, ID sanitization |

---

### Bonus: Security & CI Hardening

| File | Change |
|------|--------|
| [.trivyignore](file:///Users/james/Workspace/gh/application/vitruvian/devx/.trivyignore) | Suppress `golang.org/x/crypto/ssh` CVEs (transitive, never imported by devx) |
| [audit.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/audit/audit.go) | Added `--ignorefile` support to Trivy for both native and container modes |
| [go.mod.tmpl](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/scaffold/templates/go-api/go.mod.tmpl) | Bumped chi v5.1.0 → v5.2.5 (GHSA-vrw8-fxc6-2r93) |
| `.gitignore` | Added `cmd/var/` to prevent test fixture commits |

## Validation

- ✅ `go build ./...` — compiles cleanly
- ✅ `go test ./... -count=1` — all packages pass
- ✅ `go vet ./...` — no issues
- ✅ Gitleaks scan — PASS (no secrets detected)
- ✅ Trivy scan — PASS (CVEs resolved or suppressed)
- ✅ Push to origin — successful
