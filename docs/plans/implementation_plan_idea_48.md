# Idea 48 — Seed Data Runner (`devx db seed`)

The `devx db seed` command minimizes friction when populating local databases with realistic test data. Many projects rely on seed scripts (Node.js Prisma seeders, Django fixtures, Go CLI seeders), each requiring developers to manually figure out connection strings, ports, and environment variables.

This feature allows developers to specify a `seed.command` in `devx.yaml`. The CLI will automatically resolve the active database container's connection string, inject it (and all individual credential fragments) into the command's environment, and execute the script on the host machine.

## User Review Required

> [!WARNING]
> **Execution environment:** The seed command will execute on the **host machine**, not inside the database container. This is intentional — seed scripts typically depend on host tools (e.g., `npm run seed`, `go run ./cmd/seed`, `python manage.py loaddata`). The database is reached via `localhost:<mapped-port>` which Podman already forwards.

> [!IMPORTANT]
> **Injected environment variables:** Both `DATABASE_URL` (connection URI) and all individual fragment variables from `database.Engine.Env` (e.g. `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`) will be injected, plus two additional computed variables:
> - `DATABASE_HOST` → always `localhost`
> - `DATABASE_PORT` → the **actual host-mapped port** (which may differ from the default if the user started on a non-standard port or got auto-bumped by port collision recovery)

## Proposed Changes

### Configuration

#### [MODIFY] [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example)
- Add `seed:` section under the postgres database block, adjacent to the existing `pull:` section:

```yaml
databases:
  - engine: postgres
    port: 5432
    pull:
      command: "gcloud storage cat gs://acme-dumps/nightly.sql.gz | gunzip"
    seed:
      command: "npm run db:seed"
      # Or: "go run ./cmd/seed"
      # Or: "python manage.py loaddata fixtures/*.json"
```

#### [MODIFY] [devx.yaml](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml)
- Add a commented-out `seed:` block in the project's own config for dogfooding reference.

---

### Command Implementation

#### [NEW] [db_seed.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/db_seed.go)

**Design decisions and gap fixes vs. previous plan:**

1. **Dynamic host port resolution** — The previous plan said "retrieve the dynamically mapped port" but didn't specify how. `db_pull.go` doesn't resolve the actual host port either; it assumes the default. This is a bug-in-waiting because `db_spawn.go` has port-collision auto-increment logic (lines 100–114). I will resolve the actual host port via `podman inspect --format '{ { (index (index .NetworkSettings.Ports "<internal>/tcp") 0).HostPort } }' devx-db-<engine>`. This gives us the *real* mapped port regardless of auto-bumps.

2. **Consistent YAML parsing** — Mirror the `dbPullYAML` struct pattern from `db_pull.go` but add a `Seed` field:
```go
type dbSeedYAML struct {
    Databases []struct {
        Engine string `yaml:"engine"`
        Seed   struct {
            Command string `yaml:"command"`
        } `yaml:"seed"`
    } `yaml:"databases"`
}
```

3. **Full environment injection** — The seed subprocess inherits `os.Environ()` and we layer on:
   - `DATABASE_URL` → `engine.ConnString(resolvedPort)` (formatted URI)
   - `DATABASE_HOST` → `localhost`
   - `DATABASE_PORT` → the resolved host port (string)
   - All key/value pairs from `database.Engine.Env` (e.g., `POSTGRES_USER=devx`, `POSTGRES_PASSWORD=devx`, `POSTGRES_DB=devx`)

4. **Global flag compliance** — Honor `DryRun` (print what would execute without running), `NonInteractive` (skip `huh.Confirm`), and `--json` (structured output for AI agents), matching the patterns established across 15+ existing commands.

5. **--runtime flag** — Local flag for runtime override, consistent with `db_pull.go` and `db_spawn.go`.

6. **Stderr + stdout passthrough** — The seed command's stdout/stderr are piped directly to the terminal so developers see real-time feedback from their seed scripts (migration logs, row counts, errors).

7. **gcloud auth recovery** — If the seed command itself fails due to a gcloud auth issue (e.g., it pulls fixtures from GCS), we intercept the error via `devxerr.RecoverGcloudAuth` and offer auto-retry — consistent with the pattern just shipped in `devx ci run`.

#### [MODIFY] [db.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/db.go)
- `dbSeedCmd` is already wired via `init()` in `db_seed.go` calling `dbCmd.AddCommand(dbSeedCmd)` — no changes to `db.go` itself. This matches the pattern used by `db_pull.go`, `db_rm.go`, `db_spawn.go`, etc.

---

### Documentation (mandatory post-verification)

#### [MODIFY] [databases.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/databases.md)
- Add a full `### devx db seed` section between `### devx db pull` and `### devx db snapshot`, documenting:
  - Purpose and usage examples
  - Configuration syntax in `devx.yaml`
  - Full list of injected environment variables with a table
  - Flags reference

#### [MODIFY] [IDEAS.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/IDEAS.md)
- Remove Idea 48 from the P3 backlog.

#### [MODIFY] [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md)
- Add Idea 48 to the completed features section under "Developer Productivity & Extensibility".

#### [MODIFY] [README.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/README.md)
- Add a bullet for seed data runner in the feature list if not already present.

## Open Questions

None at this time.

## Verification Plan

### Automated Tests
- N/A for CLI orchestration layer (no integration test harness for container lifecycle).

### Manual Verification
1. Spawn a Postgres DB via `devx db spawn postgres`.
2. Create a test seed script that echoes all injected variables:
   ```bash
   #!/bin/bash
   echo "DATABASE_URL=$DATABASE_URL"
   echo "DATABASE_HOST=$DATABASE_HOST"
   echo "DATABASE_PORT=$DATABASE_PORT"
   echo "POSTGRES_USER=$POSTGRES_USER"
   echo "POSTGRES_PASSWORD=$POSTGRES_PASSWORD"
   echo "POSTGRES_DB=$POSTGRES_DB"
   ```
3. Configure `seed.command` in `devx.yaml` pointing to the script.
4. Run `devx db seed postgres` — verify all variables are populated with correct values, especially `DATABASE_PORT` matching the actual mapped port.
5. Run `devx db seed postgres --dry-run` — verify it prints the planned action without executing.
6. Run `devx db seed postgres -y` — verify it skips confirmation.
7. Run `devx db seed postgres --json` — verify structured output.
8. Test with a non-standard port by spawning on a different port (`devx db spawn postgres -p 5433`) and verifying `DATABASE_PORT=5433`.
9. **Update official documentation** (`docs/guide/databases.md`, `FEATURES.md`, `IDEAS.md`, `README.md`) and verify VitePress builds cleanly with `npm run docs:build`.
10. Ship via `devx agent ship` and cut a release.
