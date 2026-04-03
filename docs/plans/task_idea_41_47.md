# Shareable Diagnostic Dumps & Time-Travel Checkpoints (Ideas 41 & 47) - Task Tracker

## Phase 1: Infrastructure & Package Setup
- [x] Create `internal/state` package
- [x] Implement `dump.go` in `internal/state`
- [x] Implement `checkpoint.go` in `internal/state`

## Phase 2: CLI Commands (Dump)
- [x] Create `cmd/state.go` group command
- [x] Create `cmd/state_dump.go` implementation (Markdown format, JSON export, config redaction, file output)

## Phase 3: CLI Commands (Checkpoint/Restore)
- [x] Create `cmd/state_checkpoint.go`
- [x] Create `cmd/state_restore.go`
- [x] Create `cmd/state_list.go` / `cmd/state_rm.go`

## Phase 4: Verification & Documentation
- [x] Update documentation (`docs/guide/`) for new `state` command capabilities.
- [x] Verify `devx state dump` redacts secrets and extracts stack correctly.
- [x] Verify `devx state checkpoint` and `restore` work cleanly.
- [ ] Dogfooding: Ship implementation via `devx agent ship`!
