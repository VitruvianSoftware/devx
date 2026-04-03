# Idea 44: Unified Multirepo Orchestration

## Problem Statement

Running a company's full-stack infrastructure locally means juggling 5–10 separate repository directories. Each has its own `devx.yaml` orchestrating its local services, databases, and tunnels — but there's no mechanism to compose them into a single unified DAG. Developers manually `cd` between repos running `devx up` in multiple terminals, then fight "Connection Refused" errors because services in repo A can't discover databases started by repo B.

## Design Decisions

1. **Namespace isolation strategy: Fail-Fast on duplicate names**. When two included projects (or an included project and the main project) both define a service named `api` or a database named `postgres`, we will error immediately with: `Conflict: service "api" defined in both ./api/devx.yaml and ./web/devx.yaml. Rename one or use the parent devx.yaml to resolve.` Failing loudly forces users to define unique, descriptive service names (docker-compose approach) and avoids hidden complexity with healthcheck URLs and dependent environment variables.

2. **Port collision handling: Auto-shift across includes**. If multiple inclusive projects attempt to expose the same port (e.g. `postgres: port 5432`), the existing port-shifting logic from Idea 36 will auto-bump the subsequent instances and print a visible warning: `⚠️ Port 5432 (users/postgres) shifted to 5433 — collides with payments/postgres`. This provides a frictionless unified cluster experience.

3. **`include` lives at the top level of `DevxConfig`, not in services.** Includes are a project-composition concern, not a service-level concern. This mirrors Docker Compose's design. The `include` block is processed *before* profile merging, so profiles can overlay included services.

2. **Relative path resolution uses the includer's directory as base.** `include: ["../payments/devx.yaml"]` resolves relative to the directory containing the parent `devx.yaml`. Each included file's services have their commands executed with `cmd.Dir` set to that included file's directory — this is **critical** because commands like `go run ./cmd/api` are path-relative.

3. **Included projects are flattened, not nested.** All databases, services, tunnels, and mocks from all included files are merged into a single flat `DevxConfig`. There are no sub-projects at runtime — the merged config is indistinguishable from a manually-written monolithic `devx.yaml`. This keeps the DAG, nuke, sync, and all other features unmodified.

4. **Recursive includes are supported (depth-limited).** An included `devx.yaml` can itself contain an `include` block. To prevent infinite loops, recursion is capped at depth 5. Each file is tracked by absolute path; re-including the same file is a no-op (deduplicated silently).

5. **`env_file` override per include.** Each include entry can optionally specify an `env_file` path for that sub-project's secret resolution. This prevents leaking the parent's `.env` into child projects.

6. **Working directory tracking.** Each `DevxConfigService` gets a new field `Dir string` (yaml: `dir`, internal-only, not user-facing). The `include` parser sets this to the absolute directory of the included `devx.yaml`. The `orchestrator.Node` struct gets a matching `Dir` field, and `startHostProcess` uses it as `cmd.Dir`.

## Gap Analysis

1. **`startHostProcess` in `dag.go` (line 277) does not set `cmd.Dir`.** Today all services run from the user's CWD, which works because `devx up` is run from the project root. With includes, commands like `go run ./cmd/api` from a sibling repo would fail because the CWD is wrong. **Fix:** Add `cmd.Dir = n.Dir` after constructing the command. When `Dir` is empty (single-project mode), it falls back to the current behavior (inherits parent's CWD).

2. **YAML Parsing Fragmentation (Major Architecture Gap)**: Currently, `cmd/up.go`, `cmd/db_seed.go`, `cmd/map.go`, `cmd/test_ui.go`, `cmd/sync_up.go`, `cmd/config_validate.go` all manually read `devx.yaml` from disk and parse it. Further, `db_seed.go` uses a custom anonymous struct because the core `DevxConfigDatabase` struct in `up.go` is missing the `seed` and `pull` schemas.
   - **Fix**: We must fully flesh out `DevxConfig` and implement a single, unified `resolveConfig(path, profile)` function inside `package cmd` (`cmd/config.go`). All commands must switch to this central resolver so that _every_ command instantly supports multirepo `include` blocks.

3. **Database Working Directory**: `devx db seed postgres` executes commands on the host (e.g. `npm run db:seed`). If the database was defined in an included project, the seed command must run from that included project's directory.
   - **Fix**: Add a `Dir string` field to `DevxConfigDatabase` (just like `Service`), populated by the include resolver.

4. **The nuke collector scans only the *current* directory.** With multirepo, databases and containers spawned by included projects would be missed by `devx nuke`. However, since we use label-based container discovery (`managed-by=devx`), containers are found regardless of which project spawned them. Filesystem caches (`node_modules`, etc.) in sibling repos would not be cleaned. **Acceptable trade-off** — developers can run `devx nuke` in each repo individually for filesystem cleanup. Container/volume cleanup is global.

## Proposed Changes

### Config Schema

#### [MODIFY] [cmd/up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go)

**1. Add `Include` field and `Dir` field:**

```go
// DevxConfigInclude defines an external devx.yaml to compose into this project.
type DevxConfigInclude struct {
    Path    string `yaml:"path"`     // Path to devx.yaml (relative to this file's directory)
    EnvFile string `yaml:"env_file"` // Optional .env override for the included project
}

// Add to DevxConfigService:
type DevxConfigService struct {
    // ... existing fields ...
    Dir  string `yaml:"-"` // Internal: working directory for command execution (set by include resolver)
}

// Add to DevxConfig:
type DevxConfig struct {
    // ... existing fields ...
    Include []DevxConfigInclude `yaml:"include"` // External devx.yaml files to compose
}
```

**2. Add include resolver logic** — new function `resolveIncludes(cfg *DevxConfig, baseDir string, depth int, seen map[string]bool) error`:

- Reads each `include[].path` relative to `baseDir`
- Parses the included `devx.yaml` into a `DevxConfig`
- Sets `Dir` on every service in the included config to the directory of the included file
- Recursively resolves includes up to depth 5
- Deduplicates by absolute path via the `seen` map
- Merges databases, tunnels, services, and mocks into the parent config (append, no override)
- Detects name collisions and errors immediately (fail-fast)

**3. Integrate into `devx up RunE`** — call `resolveIncludes` after YAML parsing but before profile merging (between lines 137–139):

```go
if len(cfgYaml.Include) > 0 {
    baseDir := filepath.Dir(yamlPath)
    if err := resolveIncludes(&cfgYaml, baseDir, 0, map[string]bool{}); err != nil {
        return fmt.Errorf("include resolution failed: %w", err)
    }
}
```

**4. Pass `Dir` through to DAG nodes** — when constructing `orchestrator.Node` (line 353), add:

```go
Dir: svc.Dir,
```

---

### Config Resolution Library (Centralization)

#### [NEW] [cmd/config.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/config.go)

Extract `DevxConfig`, `DevxConfigService`, etc., out of `up.go` and into this new file. Add the missing `Seed` and `Pull` structs to `DevxConfigDatabase`.

Implement the unified include processing:

```go
// resolveConfig reads a devx.yaml, processes all include directives recursively 
// (setting .Dir on services and databases), and applies the profile overlay.
func resolveConfig(yamlPath, profile string) (*DevxConfig, error)
```

#### [MODIFY] Multiple CLI Commands

Refactor the following commands to delete their manual `os.ReadFile("devx.yaml")` / `yaml.Unmarshal` blocks and replace them with `cfg, err := resolveConfig("devx.yaml", profile)` (where `profile` might be empty):
- `cmd/up.go`
- `cmd/sync_up.go`
- `cmd/db_seed.go`
- `cmd/test_ui.go`
- `cmd/map.go`
- `cmd/config_pull.go`
- `cmd/config_push.go`
- `cmd/config_validate.go`

---

### Orchestrator: Working Directory Support

#### [MODIFY] [internal/orchestrator/dag.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/orchestrator/dag.go)

**1. Add `Dir` to `Node` struct** (line 74–87):

```go
type Node struct {
    // ... existing fields ...
    Dir     string // Working directory for host process execution
}
```

**2. Set `cmd.Dir` in `startHostProcess`** (after line 277):

```go
cmd := exec.CommandContext(childCtx, n.Command[0], n.Command[1:]...)
if n.Dir != "" {
    cmd.Dir = n.Dir
}
```

---

### Sync: Shared Config Resolution

#### [MODIFY] [cmd/sync_up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/sync_up.go)

Replace the standalone YAML parsing (lines 63–71) with a call to the same config resolution path used by `devx up`. This ensures `devx sync up` sees included projects' sync blocks.

---

### devx.yaml.example

#### [MODIFY] [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example)

Add an `include` example at the top:

```yaml
# --- Idea 44: Unified Multirepo Orchestration ---
# Compose multiple sibling repositories into a single orchestrated topology.
# Each included project's commands execute from its own directory.
# include:
#   - path: ../payments-api/devx.yaml
#   - path: ../user-service/devx.yaml
#     env_file: ../user-service/.env.local
```

---

### VitePress Documentation

#### [NEW] [docs/guide/multirepo.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/multirepo.md)

Full documentation page covering:
- Problem statement (multirepo friction)
- Quick start (`include` directive usage)
- Short and long syntax
- Working directory behavior
- Cross-project dependencies
- Port collision handling
- Recursive includes and deduplication
- Troubleshooting (name collisions, circular includes, missing files)

#### [MODIFY] [docs/.vitepress/config.mjs](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/.vitepress/config.mjs)

Add sidebar entry after "Smart File Syncing":

```js
{ text: 'Multirepo Orchestration', link: '/guide/multirepo' },
```

#### [MODIFY] [README.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/README.md)

Add feature bullet: `🌐 Multirepo Orchestration — compose multiple devx.yaml files from sibling repositories into a single unified dev environment`

#### [MODIFY] [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md)

Move Idea 44 to completed features.

#### [MODIFY] [IDEAS.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/IDEAS.md)

Remove Idea 44 from backlog.

## Open Questions

> [!IMPORTANT]
> **Namespace strategy:** Confirm fail-fast on duplicate names (Option 2) from User Review section above.

> [!IMPORTANT]
> **Port collision handling:** Confirm auto-shift with warning (recommended) from User Review section above.

## Verification Plan

### Build Validation
1. `go build ./...` — ensure all changes compile
2. `go vet ./...` — static analysis
3. `go test ./...` — existing tests pass
4. `npm run docs:build` (in `docs/`) — VitePress builds with new `multirepo.md`

### Happy Path
1. Create a test layout with 2 directories, each with their own `devx.yaml`:
   - `/tmp/multirepo-test/orchestrator/devx.yaml` (parent, includes both)
   - `/tmp/multirepo-test/svc-a/devx.yaml` (service "alpha" on port 9090)
   - `/tmp/multirepo-test/svc-b/devx.yaml` (service "beta" on port 9091)
2. Run `devx up --dry-run` from orchestrator → verify all services shown (this feature doesn't exist yet, so we'll verify via config parsing output)
3. Verify services print their correct `Dir` in startup logs

### Edge Cases
1. **Duplicate service name across includes:** Create two includes both defining `api`. Run `devx up` → verify fail-fast error: `Conflict: service "api" defined in both...`
2. **Duplicate port across includes:** Two includes both use port 5432 → verify port-shift warning: `⚠️ Port 5432... shifted to 5433`
3. **Missing include path:** Reference a non-existent path → verify error: `include resolution failed: cannot read ../missing/devx.yaml: no such file`
4. **Circular include:** A includes B, B includes A → verify error: `include depth exceeded maximum (5)` or deduplication silently ignores the cycle
5. **`devx sync up` with includes:** Add sync blocks to an included project → verify `devx sync up --dry-run` discovers them
6. **Service CWD:** Included service runs `echo $PWD` → verify it prints the included project's directory, not the parent's

### Documentation (MANDATORY — after all verifications pass)
1. Create `docs/guide/multirepo.md`
2. Update `docs/.vitepress/config.mjs` sidebar
3. Update `README.md` feature list
4. Move Idea 44: `IDEAS.md` → `FEATURES.md`
5. Run `npm run docs:build` — final verification
