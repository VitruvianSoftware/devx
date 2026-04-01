# Databases

The `devx db` commands let you spin up ephemeral, persistent, or disposable databases for local development — no Docker Compose files needed.

## Commands

### `devx db spawn`

Spin up a database with persistent storage:

```bash
# PostgreSQL
devx db spawn postgres --name myapp-db

# MySQL
devx db spawn mysql --name myapp-mysql

# Redis
devx db spawn redis --name cache
```

Each database runs as a Podman container with a named volume for data persistence. Credentials are printed to stdout on creation.

**Flags:**

| Flag | Description |
|------|-------------|
| `--name` | Name for the database container |
| `--port` | Host port to bind (auto-assigned if omitted) |
| `--version` | Database version tag (e.g., `16`, `8.0`) |

### `devx db list`

Show all devx-managed databases and their connection strings:

```bash
devx db list
```

```
NAME           ENGINE     PORT    STATUS    VOLUME
myapp-db       postgres   5432    running   devx-myapp-db
myapp-mysql    mysql      3306    running   devx-myapp-mysql
cache          redis      6379    running   devx-cache
```

### `devx db rm`

Stop and remove a database container. Data volumes are preserved by default:

```bash
devx db rm myapp-db              # Keep the volume
devx db rm myapp-db --volumes    # Delete everything
```

### `devx db pull`

Pull a pre-scrubbed production/staging dataset and stream it directly into your local container:

```bash
devx db pull postgres
```

#### Why it's secure
Dumping real production databases locally is a massive security risk. Instead of connecting your laptop directly to production or pulling unscrubbed PII, `devx` delegates to a shell command defined in your `devx.yaml`. 

The standard approach is configuring your cloud environment to generate a nightly, anonymized dump (scrubbing emails, passwords, and PII) and storing it in a secure bucket. `devx db pull` simply downloads that safe artifact and pipes it directly into the container's ingestion tool (`psql`, `mysql`, etc.) without writing massive temporary files to your disk.

#### Configuration

Add a `pull.command` to your `devx.yaml`:

```yaml
databases:
  - engine: postgres
    port: 5432
    pull:
      # A shell command that outputs raw SQL to stdout
      command: "gcloud storage cat gs://acme-scrubbed-dumps/nightly.sql.gz | gunzip"
```

When you run `devx db pull postgres`, `devx` will:
1. Ensure the `postgres` container is running.
2. Prompt for confirmation (unless `-y` is passed).
3. Execute your `command` and pipe it directly into `podman exec -i devx-db-postgres psql -U devx -d devx`.

::: warning Drop commands
Ensure your SQL dump includes `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` or `DROP TABLE IF EXISTS` statements. `devx` streams the dump sequentially; it does not automatically drop the database beforehand. This prevents accidental data loss if the pull command fails midway or lacks data.
:::

### `devx db snapshot`

Create, restore, list, and delete zero-SQL point-in-time snapshots of devx-managed volumes. Snapshots are stored as ultra-fast compressed tar archives in `~/.devx/snapshots/`.


Useful before running destructive migrations or testing complex state changes — restore to a known-good state in seconds without re-running SQL seed scripts.

```bash
# Export the current state of your local database
devx db snapshot create postgres before-migration

# Run your destructive migrations...
# Reset back to the clean snapshot in seconds
devx db snapshot restore postgres before-migration

# View existing snapshots and their sizes
devx db snapshot list postgres

# Clean up
devx db snapshot rm postgres before-migration
```

Snapshots respect `--json` output for AI agents, and destructive restorations ask for confirmation unless bypassed with `-y`.

## Declarative Mode

For projects with multiple services, define your databases in `devx.yaml`:

```yaml
databases:
  - engine: postgres
    name: api-db
    port: 5432
    version: "16"
  - engine: redis
    name: cache
    port: 6379
```

Then provision everything at once:

```bash
devx up
```

## Connection Strings

After spawning, `devx db list` shows connection strings you can copy directly into your `.env`:

```bash
DATABASE_URL=postgres://devx:devx@localhost:5432/devx
REDIS_URL=redis://localhost:6379
```
