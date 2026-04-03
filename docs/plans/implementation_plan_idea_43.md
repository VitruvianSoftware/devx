# Idea 43: Smart File Syncing (Zero-Rebuild Hot Reloading)

## Problem Statement

Hot reloading using OS-level volume mounts on macOS (VirtioFS) is catastrophically slow for file trees with thousands of entries (e.g., `node_modules`). Rebuilding full container images to inject code changes disrupts developer flow state entirely. Idea 43 proposes wrapping Mutagen as a first-class sync engine behind the `devx sync` subcommand.

## User Review Required

> [!IMPORTANT]
> **Mutagen has NO native Podman support.** Mutagen's `docker://` transport shells out to the `docker` CLI binary for `docker exec` and `docker cp`. It does **not** support `podman://`. Two options:
>
> 1. **Set `DOCKER_HOST` to point at Podman's Docker-compatible socket** (`unix://$HOME/.local/share/containers/podman/machine/podman.sock`). Mutagen will then talk to Podman via Docker API compatibility. This works for rootful containers but has known failures with rootless Podman due to user namespace path-length limitations.
> 2. **Require Docker CLI installation alongside Podman** — `devx doctor` already tracks Docker as an optional tool.
>
> **My recommendation:** Option 1 (auto-set `DOCKER_HOST` when `--runtime=podman`). We control the environment when spawning `mutagen` subprocess, so we can inject this transparently. If it fails, we surface a clear error pointing to `devx doctor install --all` for Docker CLI as fallback.
> **Please confirm.**

> [!WARNING]
> **`devx up` integration:** Should `devx up` automatically start sync sessions after the DAG finishes spinning containers? **My recommendation:** No. Mutagen spawns a persistent background daemon (`mutagen daemon start`) and background sync sessions that survive terminal exit. Implicitly hijacking `devx up` with persistent daemon side-effects would surprise developers. Instead, `devx up` should print a hint: `💡 Tip: Run 'devx sync up' to enable zero-rebuild hot reloading for container services.` This is printed only when sync blocks are detected in devx.yaml.

## Design Decisions

1. **Mutagen over custom implementation.** The IDEAS.md explicitly warns: "File sync bugs are silent data corruptors — ship it buggy and you destroy developer trust." Mutagen handles rename cascades, symlinks, `.gitignore` rules, and permission mapping. Building this from scratch is months of work; wrapping Mutagen is days.

2. **Container name convention.** For services with `runtime: container`, `devx` already controls the container name via `devx shell`'s pattern: `devx-shell-<basename>`. However, `devx up` orchestrator uses `orchestrator.Node` which launches host processes, not containers. When `runtime: container`, the `Command` field contains the raw docker/podman invocation. **We cannot infer the container name from this.** Therefore, sync blocks will require an explicit `container` field specifying the target container name/ID.

3. **Sync block lives at service level, not top-level.** Sync mappings are inherently per-service (each service maps different paths into different containers). Nesting `sync` under `DevxConfigService` mirrors how `env`, `depends_on`, and `healthcheck` already work.

4. **Default ignores.** Every sync session will automatically exclude `.git`, `node_modules`, `.devx`, `__pycache__`, `.next`, `.nuxt`, `dist`, `build` unless overridden. This prevents catastrophic performance degradation from syncing millions of files.

5. **Session naming convention.** All Mutagen sessions are named `devx-sync-<service-name>` for idempotent create/terminate operations and to avoid collisions with user-created sessions.

## Gap Analysis

1. **`nuke.go` does not clean up Mutagen sessions.** The nuke manifest (`internal/nuke/`) currently scans for containers, volumes, and filesystem caches. It has no awareness of Mutagen sync sessions. If a developer runs `devx nuke`, orphaned Mutagen sessions would persist indefinitely. **Fix:** Add a `mutagen` category to the nuke collector.

2. **Profile merging does not handle sync blocks.** `mergeProfile()` in `up.go` merges databases, tunnels, services, and mocks by name. Since `sync` is nested inside services, existing service merge logic already handles it via the full service override. **No additional merge logic needed** — if a profile overrides a service, its sync block is replaced wholesale.

3. **`devx doctor` Feature Readiness table (line 246-261 in `doctor.go`) does not include `devx sync`.** A `devx sync up` readiness check should be added: requires `mutagen` binary.

4. **The `--runtime` flag exists per-command** (e.g., `db_spawn.go` line 37, `shell.go` line 31, `mock_up.go` line 41) rather than as a global persistent flag. Sync commands need their own `--runtime` flag following this same pattern.

## Proposed Changes

### Configuration Schema

#### [MODIFY] [up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go)

Add `DevxConfigSync` struct and embed it in `DevxConfigService`:

```go
// DevxConfigSync defines a host→container file sync mapping.
type DevxConfigSync struct {
    Container string   `yaml:"container"` // Target container name (e.g., "devx-shell-api")
    Src       string   `yaml:"src"`       // Host source path (relative to devx.yaml)
    Dest      string   `yaml:"dest"`      // Container destination path
    Ignore    []string `yaml:"ignore"`    // Additional ignore patterns (on top of defaults)
}
```

Add `Sync []DevxConfigSync` field to `DevxConfigService` (line 72-79).

Add a post-DAG hint in `devx up` (after line 370): if any service has non-empty `Sync`, print `💡 Tip: Run 'devx sync up' to enable zero-rebuild hot reloading for container services.`

#### [MODIFY] [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example)

Add a `sync:` example under the `api` service with `runtime: container`:

```yaml
  - name: api
    runtime: container
    command: ["docker", "run", "--name", "my-api", "-p", "8080:8080", "myorg/api:dev"]
    sync:
      - container: my-api
        src: ./src
        dest: /app/src
        ignore: ["*.test.ts", "coverage/"]
```

---

### Command Group: `devx sync`

#### [NEW] [cmd/sync.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/sync.go)

Root command group. Follows exact pattern of `cmd/mock.go` (18 lines):

```go
var syncCmd = &cobra.Command{
    Use:   "sync",
    Short: "Manage intelligent file synchronization into containers",
    Long:  `Bypass slow VirtioFS volume mounts by syncing file changes directly
into running containers via Mutagen. Changes propagate in milliseconds.

Sync sessions run as persistent background processes and survive terminal exit.
Use 'devx sync rm' to clean up.`,
}
```

#### [NEW] [cmd/sync_up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/sync_up.go)

Implements `devx sync up [names...]`.

**Global flag compliance:**
- `--json`: Output created sessions as JSON array `[{name, src, dest, container, status}]`
- `--dry-run`: Print the exact `mutagen sync create` commands without executing. Uses format: `[dry-run] would execute: mutagen sync create --name devx-sync-api ...`
- `-y` (`NonInteractive`): Skip confirmation prompt before creating sessions (relevant when overwriting existing sessions)
- `--runtime` (local flag, default: `podman`): Determines whether to inject `DOCKER_HOST` for Podman compatibility

**Logic flow:**
1. Parse `devx.yaml` and extract all services with non-empty `Sync` blocks
2. Filter by `[names...]` args if provided
3. Ensure Mutagen is installed (`exec.LookPath("mutagen")`). If missing, fail with: `mutagen is not installed. Run: devx doctor install --all`
4. Ensure Mutagen daemon is running (`mutagen daemon start`)
5. For each sync mapping, execute: `mutagen sync create --name devx-sync-<service> --ignore=<pattern> <src> docker://<container>/<dest>`
6. When `--runtime=podman`, set `DOCKER_HOST=unix://<podman-socket>` in the subprocess environment

**Edge cases:**
- If session already exists with same name: terminate it first, then recreate (idempotent)
- If container is not running: `mutagen sync create` will fail. Catch stderr containing "unable to connect" and print: `Container "<name>" is not running. Start it first with 'devx up' or manually.`
- If `devx.yaml` has no sync blocks: print `No sync mappings found in devx.yaml. Add 'sync:' blocks to your services.`

#### [NEW] [cmd/sync_list.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/sync_list.go)

Implements `devx sync list`.
- Executes `mutagen sync list --label-selector=managed-by=devx`
- Parses output into structured table using `tui` package
- `--json`: outputs raw `mutagen sync list` JSON

#### [NEW] [cmd/sync_rm.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/sync_rm.go)

Implements `devx sync rm [names...]`.
- If names provided: `mutagen sync terminate devx-sync-<name>` for each
- If no names: terminate all sessions matching `--label-selector=managed-by=devx`
- Honors `--dry-run` and `-y`
- If no sessions exist: print `No active sync sessions found.`

---

### Sync Engine Library

#### [NEW] [internal/sync/daemon.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/sync/daemon.go)

Thin Go wrapper around the `mutagen` CLI. Functions:

- `EnsureDaemon()` — runs `mutagen daemon start` if not already running
- `CreateSession(name, src, dest string, ignores []string, runtime string) error` — builds and executes `mutagen sync create` with proper `DOCKER_HOST` injection for Podman
- `TerminateSession(name string) error` — runs `mutagen sync terminate <name>`
- `ListSessions() ([]Session, error)` — parses `mutagen sync list --label-selector=managed-by=devx -l` output
- `DefaultIgnores() []string` — returns `[".git", "node_modules", ".devx", "__pycache__", ".next", ".nuxt", "dist", "build"]`

All functions accept a `runtime string` parameter to determine Podman socket injection.

---

### Doctor Integration

#### [MODIFY] [internal/doctor/check.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/doctor/check.go)

Add Mutagen to `allTools()` (after line 142):

```go
{
    Name:        "Mutagen",
    Binary:      "mutagen",
    FeatureArea: "File Sync",
    Required:    false,
    VersionFlag: "version",
    InstallBrew: "mutagen",
    InstallTap:  "mutagen-io/mutagen",
    Note:        "for zero-rebuild hot reloading (devx sync)",
},
```

#### [MODIFY] [cmd/doctor.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/doctor.go)

Add `devx sync up` to `computeFeatureReadiness()` (after line 260):

```go
{
    command: "devx sync up",
    ready:   tools["mutagen"],
    missing: missingList(tools, "mutagen"),
},
```

---

### Nuke Integration

#### [MODIFY] [internal/nuke/](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/nuke/) (collector)

Add a new category `"sync"` to the nuke manifest that detects active `devx-sync-*` Mutagen sessions and offers to terminate them during `devx nuke`.

---

### VitePress Documentation

#### [NEW] [docs/guide/sync.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/sync.md)

Full documentation page covering:
- Problem statement (VirtioFS performance)
- Quick start (`devx sync up`)
- `devx.yaml` sync block schema
- Default ignore patterns
- Podman compatibility notes
- Lifecycle management (`devx sync list`, `devx sync rm`)
- Troubleshooting (container not running, Mutagen not installed)

#### [MODIFY] [docs/.vitepress/config.mjs](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/.vitepress/config.mjs)

Add sidebar entry after "Local CI Emulation" (line 47):

```js
{ text: 'Smart File Syncing', link: '/guide/sync' },
```

#### [MODIFY] [README.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/README.md)

Add feature bullet: `⚡ Zero-Rebuild Hot Reloading — bypass slow VirtioFS mounts with intelligent file syncing into containers`

#### [MODIFY] [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md)

Move Idea 43 from executed section to completed features.

#### [MODIFY] [IDEAS.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/IDEAS.md)

Remove Idea 43 from backlog.

## Open Questions

> [!IMPORTANT]
> **Podman compatibility strategy:** Confirm Option 1 (auto-inject `DOCKER_HOST`) vs. Option 2 (require Docker CLI). See "User Review Required" section above.

## Verification Plan

### Build Validation
1. `go build ./...` — ensure all new files compile
2. `go vet ./...` — static analysis
3. `npm run docs:build` (in `docs/`) — ensure VitePress builds cleanly with new sync.md page

### Happy Path
1. Install Mutagen: `brew install mutagen-io/mutagen/mutagen`
2. Spawn a test container: `docker run -d --name test-sync alpine sleep 3600`
3. Create a test `devx.yaml` with a sync block pointing to `test-sync`
4. Run `devx sync up --dry-run` → verify printed command is correct
5. Run `devx sync up` → verify session is created
6. Run `devx sync list` → verify session appears in table
7. Touch a file in src → verify it appears in container within 1s
8. Run `devx sync rm` → verify session is terminated

### Edge Cases
1. **Container not running:** Stop `test-sync`, run `devx sync up` → verify actionable error message
2. **Mutagen not installed:** Temporarily rename binary, run `devx sync up` → verify install guidance
3. **No sync blocks in devx.yaml:** Remove sync blocks, run `devx sync up` → verify "no sync mappings" message
4. **Duplicate session:** Run `devx sync up` twice → verify idempotent behavior (terminate + recreate)
5. **`devx nuke` cleanup:** Create a session, run `devx nuke -y` → verify Mutagen sessions appear in manifest and are terminated
6. **`devx doctor`:** Run `devx doctor` → verify Mutagen appears in optional tools with correct install command and `devx sync up` appears in feature readiness

### Documentation (MANDATORY — after all verifications pass)
1. Create `docs/guide/sync.md`
2. Update `docs/.vitepress/config.mjs` sidebar
3. Update `README.md` feature list
4. Move Idea 43: `IDEAS.md` → `FEATURES.md`
5. Run `npm run docs:build` — final verification
