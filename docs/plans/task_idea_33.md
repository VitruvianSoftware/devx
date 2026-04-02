# Task: Idea #33 — CLI Integration Test Harness

## Phase 1: Test Infrastructure
- [ ] Create `internal/testutil/` directory
- [ ] Create `internal/testutil/fake_runtime.go`
  - Implement `$PATH` prepending logic
  - Write generic stub script that logs `$@` to a JSON file
  - Link `podman`, `docker`, and `cloudflared` to this stub

## Phase 2: Command Tests
- [ ] Create `cmd/shell_test.go`
  - Test AI bridge `-v` injection logic
  - Test `.env` file parsing to `-e` flags
  - Test standard `devx shell` execution parsing
- [ ] Create `cmd/scaffold_test.go`
  - Test basic repository clone command execution
  - Test the `--force` flag logic

## Phase 3: Verification
- [ ] Execute `go test ./cmd/... -v`
- [ ] Verify test correctness and clean outputs
- [ ] Remove Idea #33 from `IDEAS.md` and move to `FEATURES.md`
- [ ] Run the `/push` workflow
