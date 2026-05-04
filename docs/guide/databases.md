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

Pull a pre-scrubbed production/staging dataset and stream it directly into your local container — no temp files written to disk:

```bash
devx db pull postgres
```

#### Why it's secure
Dumping real production databases locally is a massive security risk. Instead of connecting your laptop directly to production or pulling raw PII, `devx` delegates to a shell command you define. The responsibility for scrubbing/anonymizing data stays in your cloud, where compliance tooling already lives.

`devx db pull` downloads that pre-scrubbed artifact and pipes it directly into the container's ingestion tool — `psql`, `mysql`, `mongorestore`, `redis-cli` — without ever writing a multi-gigabyte SQL file to your SSD.

#### Two import formats

| | **SQL (default)** | **Custom/Binary** |
|---|---|---|
| **Format** | Plain SQL text (`pg_dump`, `mysqldump`) | PostgreSQL binary (`pg_dump -Fc`) |
| **Ingestion tool** | `psql` / `mysql` | `pg_restore` |
| **Import speed** | Sequential, single-threaded | **Parallel** (uses all CPU cores) |
| **Best for** | Any engine, databases < 5 GB | Large PostgreSQL databases (5 GB+) |
| **DB engine** | All engines | PostgreSQL only |

#### Configuration

```yaml
databases:
  - engine: postgres
    pull:
      # Mode 1 — plain SQL (default): works for all engines
      command: "gcloud storage cat gs://acme-dumps/nightly.sql.gz | gunzip"

      # Mode 2 — binary format (postgres only, large databases):
      # format: custom        # switches to pg_restore instead of psql
      # jobs: 4               # parallel workers (default: auto = number of CPU cores)
      # command: "gcloud storage cat gs://acme-dumps/nightly.dump"
```

When you run `devx db pull postgres`, `devx` will:
1. Verify the `postgres` container is running (error if not).
2. Show the pull command and format, requiring confirmation (skip with `-y`).
3. Execute your `command` in a subshell and pipe stdout directly into the container.
4. For `format: custom`, use `pg_restore -j <jobs>` for parallel import.

#### Flags

| Flag | Default | Description |
|---|---|---|
| `-j, --jobs` | CPU cores | Parallel workers for `format: custom` (overrides `devx.yaml`) |
| `--runtime` | `auto-detected` | Container runtime (`podman`, `docker`, `nerdctl`) |

::: warning SQL mode: drop commands
Ensure your SQL dump includes `DROP SCHEMA public CASCADE; CREATE SCHEMA public;` or `DROP TABLE IF EXISTS` before each table. `devx` streams the dump sequentially and does not automatically drop the database first.
:::

::: tip Binary mode: pre-seed your nightly job
In GCP, create a Cloud Scheduler job that runs nightly:
```bash
pg_dump -Fc -h $PROD_HOST -U $PROD_USER mydb | \
  anonymize-pii | \
  gcloud storage cp - gs://my-company-scrubbed-dumps/nightly.dump
```
Then `devx db pull postgres` downloads and imports it in seconds with full parallelism.
:::

### `devx db seed`

While `db pull` is meant for restoring massive dumps, `devx db seed` is designed to run your application's natural seeding scripts (e.g., Prisma, Go loops, Django fixtures) directly against the local containerized database.

The tedious part of local seeding has always been figuring out which port the container mapped to, piecing together the connection string, and passing the correct arguments to your script. `devx db seed` automates this entirely.

#### Configuration

In your `devx.yaml`:

```yaml
databases:
  - engine: postgres
    seed:
      # Any command that populates your database. This command executes 
      # locally on your host environment, NOT inside the container.
      command: "npm run db:seed"
```

#### Injected Variables

When you run `devx db seed postgres`, `devx` will inspect the running `devx-db-postgres` container, resolve its correct host port, and inject a pristine set of environment variables before executing your script.

You'll get a fully formatted `DATABASE_URL` for modern ORMs, plus all standard fragments for legacy connectivity:

| Variable | Example Value | Description |
|---|---|---|
| `DATABASE_URL` | `postgresql://devx:devx@localhost:5432/devx` | Primary connection URI |
| `DATABASE_HOST` | `localhost` | Container mapped address |
| `DATABASE_PORT` | `5432` | The actual host-mapped port  |
| `POSTGRES_USER` | `devx` | Derived from `database.Engine.Env` |
| `POSTGRES_PASSWORD` | `devx` | Derived from `database.Engine.Env` |
| `POSTGRES_DB` | `devx` | Derived from `database.Engine.Env` |

If your script pulls fixtures from Google Cloud Storage and encounters an expired token, the `devx` CLI will instantly pause execution, spawn `huh.Confirm` to trigger `gcloud auth login`, and automatically retry your seed command.


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

Snapshots respect **`--json`** output for AI agents, and destructive restorations ask for confirmation unless bypassed with `-y`.

### `devx db synthesize`

Generate highly realistic, edge-case-heavy synthetic data by extracting your local database schema and sending it to an AI model. The generated SQL INSERT statements are piped directly into your running container.

```bash
# Generate 100 synthetic records for PostgreSQL
devx db synthesize postgres

# Generate 50 records using a specific model
devx db synthesize mysql --records 50 --model llama3

# Preview the schema and prompt without generating data
devx db synthesize postgres --dry-run
```

**Supported engines:** `postgres`, `mysql`

#### AI Provider Priority

`devx` automatically discovers the best available AI provider:

1. **Local Ollama** on port 11434 (zero-config if running)
2. **Local LM Studio** on port 1234
3. **OpenAI API** via `OPENAI_API_KEY` environment variable

For users running gemini-cli, claude code, codex, or opencode who already have cloud API keys, export `OPENAI_API_KEY` (or set `OPENAI_API_BASE` for custom/proxy endpoints like LiteLLM).

#### What makes the data "chaotic"?

The AI is instructed to generate intentionally adversarial data:
- **Unicode**: Japanese, Arabic, emoji, and mixed-script strings
- **Boundary values**: Empty strings, very long strings (200+ chars), NULL where nullable
- **Numeric extremes**: MIN/MAX integers, negative values, zero
- **Temporal diversity**: Dates spanning decades, edge timestamps
- **Foreign key compliance**: Parent rows are always inserted before child rows

This catches bugs that "perfect" seed data misses — things like truncation errors, encoding issues, and off-by-one date calculations.

#### Flags

| Flag | Default | Description |
|---|---|---|
| `--records` | `100` | Number of synthetic records to generate |
| `--model` | *(auto)* | Target LLM model (overrides provider default) |
| `--runtime` | `podman` | Container runtime (`podman`, `docker`) |

Supports `--json` for structured output and `--dry-run` to preview the schema and prompt without making an LLM call.

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
