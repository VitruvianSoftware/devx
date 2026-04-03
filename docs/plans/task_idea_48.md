# Implementation of Idea 48: Seed Data Runner

- [ ] Create `cmd/db_seed.go`
  - [ ] Parse `devx.yaml` for `databases[].seed.command`.
  - [ ] Retrieve dynamic host port mappings via `podman inspect` format string.
  - [ ] Compute `DATABASE_URL`, `DATABASE_HOST`, and `DATABASE_PORT`.
  - [ ] Implement command execution environment override.
  - [ ] Handle `NonInteractive`, `DryRun`, and `outputJSON` globals.
  - [ ] Inject `devxerr.RecoverGcloudAuth` retry flow.
- [x] Update `devx.yaml.example` and `devx.yaml` with seed config.
- [x] Manual verification via mock seed script.
- [x] Update Official Documentation
  - [x] `docs/guide/databases.md`
  - [x] `FEATURES.md`
  - [x] `IDEAS.md`
  - [x] `README.md`
- [x] Ship via `devx agent ship`
