# Pipeline Stages

## Overview

`devx` supports Skaffold-inspired declarative pipeline stages that let you define **exactly** how your project is tested, linted, built, and verified — regardless of your technology stack. This follows the **Familiarity-First** design principle: developers who have used Skaffold or Docker Compose should feel at home.

## Zero-Config Default

By default, `devx agent ship` **auto-detects** your stack from marker files:

| Marker File | Stack | Test | Lint | Build |
|-------------|-------|------|------|-------|
| `go.mod` | Go | `go test ./...` | `go vet ./...` | `go build ./...` |
| `package.json` | Node/JS/TS | `npm test` | `npm run lint` | `npm run build` |
| `Cargo.toml` | Rust | `cargo test` | `cargo clippy` | `cargo build` |
| `pyproject.toml` | Python | `pytest` | `ruff check .` | — |

This works out of the box with no configuration. However, when you need custom commands — or your project uses a non-standard build system — you can define explicit pipeline stages.

## Explicit Pipeline

Add a `pipeline:` block to your `devx.yaml` to override auto-detection entirely:

```yaml
pipeline:
  test:
    command: ["pytest", "-v", "--cov=app"]
  lint:
    command: ["ruff", "check", "."]
  build:
    command: ["docker", "build", "-t", "myapp", "."]
  verify:
    command: ["./scripts/integration-test.sh"]
```

::: warning EXPLICIT WINS
When a `pipeline:` block is present, auto-detection is **completely bypassed**. If you only define `build:` and `lint:`, there will be no test step — `devx` will not attempt to auto-detect a test command.
:::

### Multi-Command Stages

Need to run multiple commands in a single stage? Use `commands:` (plural) instead of `command:`:

```yaml
pipeline:
  test:
    commands:
      - ["go", "test", "./..."]
      - ["go", "vet", "./..."]
  build:
    command: ["go", "build", "./..."]
```

Each command in the list runs sequentially. If any command fails, the stage fails immediately.

### Verify Stage

The `verify:` stage is **pipeline-only** — it has no auto-detected equivalent. Use it for post-build validation like integration tests or smoke checks:

```yaml
pipeline:
  build:
    command: ["go", "build", "./..."]
  verify:
    command: ["./scripts/smoke-test.sh"]
```

## `devx run` — Universal Telemetry Wrapper

Run **any** host command with automatic telemetry:

```bash
devx run -- npm test
devx run -- go build ./...
devx run -- make deploy
```

Every `devx run` invocation:
1. Times the command execution
2. Records the exit code
3. Exports an OTel span to your local trace backend (if running)
4. Routes stdout/stderr to the devx log stream

### Global Flags
```bash
devx run --dry-run -- npm test    # prints intent without executing
devx run --name api -- go run ./cmd/api  # custom label for the log stream
```

### Viewing Telemetry

When a [distributed tracing backend](/guide/trace) is running, `devx run` spans appear alongside build metrics:

```bash
devx trace spawn grafana
devx run -- go test ./...
# Open http://localhost:3000/d/devx-build-metrics/devx-build-metrics
```

## Integration with `devx agent ship`

When you run `devx agent ship`, the pre-flight checks automatically use your pipeline configuration:

1. If `devx.yaml` has a `pipeline:` block → explicit commands are used
2. Otherwise → auto-detection from marker files (the default)

```bash
# Uses your custom pipeline
devx agent ship -m "feat: add new feature"

# Output shows pipeline source
  ▸ Phase 1: Pre-flight checks
    ℹ  using devx.yaml pipeline config
    ✓ PASS  all local checks passed (pipeline)
```
