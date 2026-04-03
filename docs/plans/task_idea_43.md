# Idea 43: Smart File Syncing — Task Checklist

## Configuration Schema
- [x] Add `DevxConfigSync` struct and embed in `DevxConfigService` in `cmd/up.go`
- [x] Add post-DAG sync hint in `devx up`
- [x] Update `devx.yaml.example` with sync block example

## Command Group: `devx sync`
- [x] Create `cmd/sync.go` (root command)
- [x] Create `cmd/sync_up.go` (start sync sessions)
- [x] Create `cmd/sync_list.go` (list active sessions)
- [x] Create `cmd/sync_rm.go` (terminate sessions)

## Sync Engine Library
- [x] Create `internal/sync/daemon.go` (Mutagen wrapper)

## Doctor Integration
- [x] Add Mutagen to `internal/doctor/check.go` allTools()
- [x] Add `devx sync up` to `cmd/doctor.go` computeFeatureReadiness()

## Nuke Integration
- [x] Add Mutagen session cleanup to nuke collector

## Build Validation
- [x] `go build ./...`
- [x] `go vet ./...`

## Manual Verification
- [x] `devx sync up --dry-run` with test devx.yaml
- [x] `devx doctor` shows mutagen in optional tools
- [x] `devx sync up` with no sync blocks → graceful message

## Documentation (MANDATORY — after verifications)
- [x] Create `docs/guide/sync.md`
- [x] Update `docs/.vitepress/config.mjs` sidebar
- [x] Update `README.md` feature list
- [x] Move Idea 43: `IDEAS.md` → `FEATURES.md`
- [x] `npm run docs:build` passes
