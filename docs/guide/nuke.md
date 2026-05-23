# The "Nuke It" Button (`devx nuke`)

`devx nuke` is a safe, confirmation-guarded hard reset for your local development environment. When caches go corrupt, containers drift, or you just need to start completely fresh — one command does it all.

## Architecture & Execution Flow

Below are the architectural component structure and the step-by-step execution flow of `devx nuke`.

### Component Diagram (C4 Level 2)

```mermaid
graph TD
    subgraph Host ["Developer Host / Environment"]
        dev["Developer / AI Agent"]
        cli["devx CLI"]
        subgraph ScanEngine ["Nuke Scan Engine"]
            langScanner["Language Cache Scanner\n(Node.js, Go, Python, Rust, Java)"]
            containerScanner["Container & Volume Scanner\n(managed-by=devx filter)"]
            sizeCalc["Disk Size Calculator"]
        end
        subgraph ManifestEngine ["Impact Manifest"]
            manifestBuilder["Manifest Builder\n(group by category + sizes)"]
            safeList["Safe List Enforcer\n(source, .env, devx.yaml, SSH, snapshots)"]
        end
        subgraph DestroyEngine ["Destroyer"]
            cacheDeleter["Cache & Artefact Deleter\n(rm -rf)"]
            containerDestroyer["Container & Volume Destroyer\n(provider-aware teardown)"]
        end
        runtime["Container Runtime\n(podman / docker / nerdctl)"]
        tui["Confirmation TUI\n(interactive prompt)"]
    end

    subgraph Project ["Project File System"]
        caches["Language Caches\n(node_modules, .next, GOPATH, etc.)"]
        devxContainers["devx-managed Containers\n& Volumes"]
    end

    dev -->|"devx nuke"| cli
    cli --> langScanner
    cli --> containerScanner
    langScanner -->|"scans"| caches
    containerScanner -->|"queries"| runtime
    runtime -->|"lists"| devxContainers
    langScanner --> sizeCalc
    containerScanner --> sizeCalc
    sizeCalc --> manifestBuilder
    safeList -->|"excludes protected items"| manifestBuilder
    manifestBuilder --> tui
    tui -->|"confirmed"| cacheDeleter
    tui -->|"confirmed"| containerDestroyer
    cacheDeleter -->|"deletes"| caches
    containerDestroyer -->|"tears down via"| runtime
```

### Execution Lifecycle Flowchart

```mermaid
flowchart TD
    Start(["devx nuke"]) --> ScanLang["Scan language ecosystems\n(Node.js, Go, Python, Rust, Java)"]
    ScanLang --> ScanContainers["Scan devx-managed containers\n& volumes (managed-by=devx)"]
    ScanContainers --> FilterExist["Filter to items that\nactually exist on disk"]
    FilterExist --> CalcSizes["Calculate disk sizes\nfor each item"]
    CalcSizes --> BuildManifest["Build impact manifest\n(grouped by category)"]
    BuildManifest --> EnforceSafe["Enforce safe list\n(source, .env, devx.yaml, SSH, snapshots)"]

    EnforceSafe --> DryRun{"--dry-run?"}
    DryRun -->|"Yes"| ShowManifest(["Display manifest\nand exit (no deletion)"])

    DryRun -->|"No"| DisplayManifest["Display manifest\nwith categories & sizes"]
    DisplayManifest --> AutoConfirm{"-y flag?"}
    AutoConfirm -->|"Yes"| Delete
    AutoConfirm -->|"No"| Confirm{"User confirms\ndeletion?"}
    Confirm -->|"No"| Cancel(["Cancelled\n(nothing deleted)"])
    Confirm -->|"Yes"| Delete

    Delete["Delete all items"] --> DeleteCaches["Remove language caches\n& build artefacts (rm -rf)"]
    Delete --> DeleteContainers["Tear down containers & volumes\n(via active --runtime provider)"]
    DeleteCaches --> Done
    DeleteContainers --> Done
    Done(["Exit 0\nNuke complete — clean slate"])
```

## What Gets Nuked

Before deleting anything, `devx nuke` **scans first and shows you exactly what will be removed** — grouped by category and with disk sizes — then asks for confirmation.

```bash
devx nuke
```

```
devx nuke — scanning project...

  The following will be permanently deleted:

  Node.js
    ✗  node_modules                              (612.4 MB)
       /Users/james/myapp/node_modules
    ✗  .next (build cache)                       (84.1 MB)
       /Users/james/myapp/.next

  Go
    ✗  module cache (GOPATH/pkg/mod/cache)       (1.1 GB)
       /Users/james/go/pkg/mod/cache
    ✗  build cache (GOCACHE)                     (863.3 MB)
       /Users/james/Library/Caches/go-build

  devx
    ✗  devx-db-postgres                          (container)
    ✗  devx-data-postgres                        (volume)
    ✗  devx-cloud-gcs                            (container)

  Total: 2.7 GB across 7 items

  Safe (never touched):
    ✓  Source code
    ✓  .env files
    ✓  devx.yaml
    ✓  SSH keys
    ✓  ~/.devx/snapshots

⚠ This cannot be undone.
Delete 7 items (2.7 GB) from your project?

Databases, containers, caches, and build artefacts will be permanently removed.
Your source code and config files (.env, devx.yaml) are safe.

[ Yes, nuke it all ] [ Cancel ]
```

## What Is Always Safe

`devx nuke` **never** touches:

| Safe | Why |
|------|-----|
| Your source code | Read-only — only caches and artefacts are removed |
| `.env` files | Secrets are yours to manage |
| `devx.yaml` | Project config is preserved |
| SSH keys | Credentials are untouched |
| `~/.devx/snapshots` | Database snapshots you created with `devx db snapshot` |

::: note
`devx nuke` respects the active `--provider` (e.g., `podman` vs `docker`) when executing container teardowns, ensuring it only cleans up containers and volumes managed by devx for that specific runtime.
:::

## Languages Supported

`devx nuke` recognises caches and build artefacts for:

| Language / Tool | What gets removed |
|---|---|
| **Node.js / JS** | `node_modules/`, `.next/`, `.nuxt/`, `dist/`, `build/`, `.turbo/`, `.parcel-cache/` |
| **Go** | `vendor/`, `GOPATH/pkg/mod/cache`, `GOCACHE` |
| **Python** | `.venv/`, `venv/`, `.pytest_cache/`, `__pycache__/` |
| **Rust** | `target/` |
| **Java / JVM** | `target/` (Maven), `build/` (Gradle) |
| **devx** | All `managed-by=devx` containers and volumes |

Only directories that **actually exist** are shown — if your project doesn't use Python, no Python entries appear.

## Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Show the manifest without deleting anything |
| `-y, --non-interactive` | Skip the confirmation prompt (for CI) |
| `--runtime` | Container runtime to use (defaults to active provider runtime e.g. `nerdctl`, `docker`, `podman`) |

## After Nuking

Once `devx nuke` completes, you'll have a completely clean slate:

```bash
devx nuke                  # Nuke everything
devx up                    # Provision fresh databases, tunnels, and containers
devx config pull           # Re-sync secrets from vault
devx config validate       # Verify all required keys are present
npm install && npm run dev # Reinstall dependencies and start your app
```

::: tip Use snapshots before nuking databases
If your database contains important test data, take a snapshot first:

```bash
devx db snapshot create postgres before-nuke
devx nuke
# Later, if needed:
devx db snapshot restore postgres before-nuke
```
:::
