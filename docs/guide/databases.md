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
