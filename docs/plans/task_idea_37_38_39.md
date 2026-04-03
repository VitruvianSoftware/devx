# P1 Polish Pass — Task Tracker

## Phase 1: Idea 37 — Environment Overlays & Profiles
- [x] Add `profiles:` schema to DevxConfig struct in `cmd/up.go`
- [x] Implement additive/merge logic for profile overrides
- [x] Add `--profile` flag to `devx up` command
- [x] Update `devx.yaml.example` with profile examples
- [x] Write unit tests for profile merging

## Phase 2: Idea 38 — Native Secrets Redaction in Logs
- [x] Create `internal/logs/redactor.go` with `SecretRedactor`
- [x] Integrate redactor into `internal/logs/streamer.go`
- [x] Wire redactor into `cmd/logs.go` (both TUI and JSON modes)
- [x] Write unit tests for redaction logic

## Phase 3: Idea 39 — Visual Architecture Map Generator
- [x] Create `cmd/map.go` with `devx map` command
- [x] Implement Mermaid flowchart generation from DAG
- [x] Register command in `cmd/root.go`
- [x] Write unit tests for Mermaid output

## Finalize
- [x] Update `devx.yaml.example` with profiles documentation
- [x] Run full test suite — PASS
- [/] Ship via /push workflow
