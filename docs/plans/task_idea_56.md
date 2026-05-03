# Task List: Idea 56 (Peer-to-Peer State Replication)

## Phase 1: Core Engine Implementation
- [x] Create `internal/state/crypto.go` (AES-256-GCM, PBKDF2/Argon2, Passphrase generation)
- [x] Create `internal/state/relay.go` (S3/GCS parsing, shell-outs, HTTP TODO breadcrumbs)
- [x] Create `internal/state/replication.go` (Tar+Gzip bundling, unbundling, cleanup)

## Phase 2: Schema & Error Codes
- [x] Modify `cmd/devxconfig.go` to add `State` and `DevxConfigState` struct
- [x] Modify `devx.yaml.example` to add commented `state:` block
- [x] Modify `internal/devxerr/error.go` to add state replication error codes (80-85)

## Phase 3: CLI Commands
- [x] Modify `cmd/state.go` to update description
- [x] Create `cmd/state_share.go` (Logic, Flags, Output Formatting)
- [x] Create `cmd/state_attach.go` (Download, Decrypt, Restore Flow, Interactive Prompts)

## Phase 4: Documentation Ecosystem Updates
- [x] Update `cmd/doctor.go` feature readiness for S3/GCS CLI requirements
- [x] Create `docs/guide/state-replication.md`
- [x] Add `devx state share/attach` to main `README.md`
- [x] Add new internal files to `CONTRIBUTING.md`
- [x] Audit and update agent skill files:
  - [x] `.agents/skills/devx/SKILL.md`
  - [x] `.agents/skills/platform-engineer/SKILL.md`
  - [x] `.github/skills/devx/SKILL.md`
  - [x] `.github/skills/platform-engineer/SKILL.md`
- [x] Migrate Idea 56 from `IDEAS.md` to `FEATURES.md`

## Phase 5: Verification & Tests
- [x] Create automated tests for `crypto.go`, `relay.go`, and `replication.go`
- [x] Update E2E test plan in `task.md`, `go vet ./...`, `staticcheck ./...`
- [x] Run `mage licensecheck`
