# Idea 45: Predictive Background Pre-Building

## Background

Per `IDEAS.md`, this feature is opt-in and high-impact only for projects with long builds. Per user direction:
- The feature ships as an **opt-in option** users can enable when their builds degrade over time.
- We add **lightweight telemetry** that proactively nudges users when builds cross the 60-second threshold.
- We update our design principles to codify **"Future-Proofing for Growth"**.

## 1. Design Decisions

### Future-Proofing for Growth (New Design Principle)
Users with small projects don't need background file-watchers. But as projects scale and builds hit minutes, wait-times break flow-state. By making Predictive Pre-Building opt-in and using telemetry to recommend it dynamically, `devx` scales its complexity alongside the user's project size.

### Telemetry is Local-Only and Non-Invasive
We already have `internal/telemetry/otel.go` for OpenTelemetry *infrastructure* (Jaeger/Grafana containers). The new `metrics.go` is a completely separate concern: it measures *devx's own* command durations on the local machine via a simple append-only JSON file at `~/.devx/metrics.json`. No data leaves the machine. No opt-in required for collection — only the nudge output is user-facing.

### The Container Build Gap (Architectural Context)
The current `devx.yaml` container runtime pattern already works: users specify the *full* `docker run` command (see `devx.yaml.example` line 138: `command: ["docker", "run", "--name", "devx-sidecar", ...]`). The orchestrator executes it via `startHostProcess()` identically to a host process. There is **no native Dockerfile build step** — users pre-build images externally.

For Idea 45 to pre-build container images, the service needs a `build:` block that tells `devx` *what* to build. This is Phase 3. Phases 1-2 deliver value immediately for **host-runtime builds** (the `go build`, `npm run build` pattern in `devx agent ship`).

### Why NOT `fsnotify` in Phase 2
The watcher daemon (Phase 3) requires `fsnotify` — a new Go dependency. Phases 1-2 have **zero new dependencies**, only stdlib `time` and `encoding/json`. This keeps the initial changeset minimal and low-risk.

## 2. Proposed Changes

---

### Phase 1: Core Principles & Telemetry Nudges
- **[MODIFY] `docs/guide/introduction.md`**
  - Add `Future-Proofing for Growth` to the core Design Principles section: ensuring the tool scales gracefully with project complexity via opt-in advanced features.

#### [NEW] `internal/telemetry/metrics.go`

**Functions:**

```go
// RecordEvent appends a timestamped duration entry to ~/.devx/metrics.json.
// Safe for concurrent use (file-level locking). Silently no-ops on any I/O error.
func RecordEvent(event string, duration time.Duration)

// NudgeIfSlow prints an actionable tip to stderr if duration exceeds threshold.
// Suppressed when jsonMode is true (for --json flag compliance).
func NudgeIfSlow(event string, duration, threshold time.Duration, jsonMode bool)
```

**Storage format** (`~/.devx/metrics.json`):
```json
[
  {"event": "agent_ship_build", "duration_ms": 72340, "timestamp": "2026-04-03T17:00:00Z"},
  {"event": "up_startup",       "duration_ms": 4500,  "timestamp": "2026-04-03T17:01:00Z"}
]
```

File is capped at 1000 entries (FIFO rotation) to prevent unbounded growth.

**Nudge output** (to stderr, suppressed in `--json` mode):
```
💡 Tip: Your build took 1m12s. Enable 'predictive_build: true' on container
   services in devx.yaml to have devx silently pre-build heavy dependency
   layers in the background. See: https://devx.dev/guide/caching
```

#### [MODIFY] `internal/ship/ship.go`

**Exact change in `RunPreFlight`** (line 113-174): Wrap the existing build step timing:

```go
// Build (existing block at line 157)
if len(stack.BuildCmd) > 0 {
    buildStart := time.Now()                              // NEW
    if err := runCmd(dir, stack.BuildCmd, verbose); err != nil {
        // ... existing error handling unchanged ...
    } else {
        result.BuildPass = true
    }
    buildDur := time.Since(buildStart)                    // NEW
    telemetry.RecordEvent("agent_ship_build", buildDur)   // NEW
    telemetry.NudgeIfSlow("build", buildDur,              // NEW
        60*time.Second, false)                             // NEW
}
```

> [!NOTE]
> The `NudgeIfSlow` call in `ship.go` cannot access the global `outputJSON` flag from the `cmd` package (it's in `internal/ship`). We need to thread the `jsonMode` bool through `RunPreFlight`'s signature or accept false here (ship is always human-facing in practice since `devx agent ship --json` already suppresses TUI output at the `cmd` layer). The simpler approach is to add `JSONMode bool` to the existing `Options` struct that `RunPreFlight` doesn't currently take, **or** accept that `RunPreFlight` is always called with `verbose` and the nudge goes to stderr which is already suppressed. I'll use the latter — stderr nudges are acceptable.

#### [MODIFY] `cmd/up.go`

**Exact change in `runE`**: Wrap the `dag.Execute()` call (around line 266) with timing:

```go
dagStart := time.Now()
dagCleanup, err = dag.Execute(ctx)
dagDur := time.Since(dagStart)
telemetry.RecordEvent("up_startup", dagDur)
```

No nudge here — `devx up` startup time is not a "build" in the traditional sense. The nudge only fires for explicit build commands.

---

### Phase 2: The `devx stats` Command

#### [NEW] `cmd/stats.go`

A simple read-only command that parses `~/.devx/metrics.json` and outputs P50/P90/P99 latency per event type.

```
$ devx stats

📊 devx local metrics (last 30 days)

  Event               Count   P50       P90       P99
  ─────────────────── ─────── ───────── ───────── ─────────
  agent_ship_build    47      8.2s      42.1s     1m12s
  up_startup          23      3.1s      5.8s      12.4s

  Data: ~/.devx/metrics.json (832 entries)
```

**Flags:**
- `--json` → machine-readable output
- `--clear` → truncate the metrics file

---

### Phase 3: Native Container Builds + Predictive Watcher (Deferred)

> [!IMPORTANT]
> Phase 3 requires adding `fsnotify` as a new Go dependency (`go get github.com/fsnotify/fsnotify`) and introducing a `build:` schema extension to `DevxConfigService`. This is the "big lift" and should be a separate PR after Phases 1-2 ship and we have real telemetry data on build times.

#### Schema Extension (Preview)

```yaml
services:
  - name: api
    runtime: container
    build:
      dockerfile: ./Dockerfile
      context: .
    predictive_build: true   # opt-in: watch go.mod/package.json and pre-build
    command: ["api-server"]
    port: 8080
```

#### [MODIFY] `cmd/devxconfig.go`
- Add `DevxConfigBuild` struct: `Dockerfile string`, `Context string`
- Add `Build DevxConfigBuild` and `PredictiveBuild bool` to `DevxConfigService`

#### [NEW] `internal/build/watcher.go`
- Uses `fsnotify` to watch dependency manifests in the service's `Dir`
- Debounces rapid saves (500ms)
- Silently triggers `podman build -t devx-<name>:prebuild -f <Dockerfile> <Context>` in background
- Respects `--dry-run` (log what would happen, don't build)
- Logs to `~/.devx/logs/prebuild-<name>.log`

#### [MODIFY] `internal/orchestrator/dag.go`
- Add `Build` and `PredictiveBuild` fields to `Node`
- In `Execute()`: if `Runtime == container` and `Build.Dockerfile != ""`, run `podman build` before `podman run`
- After all services start: spawn watcher goroutines for services with `PredictiveBuild: true`

## 3. Global Flags Compliance

| Flag | Phase 1-2 Behavior | Phase 3 Behavior |
|------|---------------------|-------------------|
| `--dry-run` | Metrics still recorded (no side effects) | Watcher logs "would build" but doesn't execute podman |
| `--json` | Nudge suppressed; `devx stats --json` outputs JSON | Watcher status included in DAG JSON output |
| `-y` | No impact (no prompts) | No impact |
| `--runtime` | No impact | Container runtime (`podman`/`docker`) used for builds |

## 4. Edge Cases

| Scenario | Handling |
|----------|----------|
| `~/.devx/metrics.json` corrupted or unreadable | `RecordEvent` silently no-ops; `devx stats` prints "no data" |
| Metrics file exceeds 1000 entries | FIFO rotation: oldest entries trimmed on next write |
| Build takes < 1 second | Recorded but no nudge (threshold is 60s) |
| `devx agent ship` run in CI (no terminal) | Nudge to stderr is harmless; CI captures it in logs |
| Multiple concurrent `devx` processes writing metrics | File-level locking via `flock` (Go `syscall.Flock`) on Darwin/Linux |
| `devx nuke` | Does NOT delete `~/.devx/metrics.json` — telemetry is user-level, not project-level |

## 5. Error Handling

- `RecordEvent` and `NudgeIfSlow` are **fire-and-forget**. They must never cause a command to fail. All I/O errors are swallowed silently.
- `devx stats --clear` prompts for confirmation unless `-y` is passed.

## 6. Documentation Updates (After Successful Verification)

| Document | Change |
|----------|--------|
| `docs/guide/introduction.md` | Add "Future-Proofing for Growth" to design principles |
| `devx.yaml.example` | Add commented `predictive_build: true` example on a container service |
| `docs/guide/caching.md` (New) | Explains the predictive build feature, when to enable it, and how the telemetry nudge works |
| `docs/.vitepress/config.mjs` | Add sidebar entry for caching guide |
| `README.md` | Add feature bullet for predictive pre-building |
| `IDEAS.md` → `FEATURES.md` | Migrate Idea 45 entry after shipping |
| `FEATURES.md` | Add Idea 45 completed entry |

## 7. Verification Plan

### Automated Tests
- `go vet ./...` — clean
- `go test ./...` — all pass
- **[NEW] `internal/telemetry/metrics_test.go`**: Test `RecordEvent` writes, FIFO rotation at 1000 entries, `NudgeIfSlow` threshold behavior, corrupted file recovery
- **[NEW] `cmd/stats_test.go`**: Test percentile calculation, empty-data output, `--json` format

### Edge Case Scenarios
- Verify nudge is suppressed when `--json` is active
- Verify `devx nuke` does not delete `~/.devx/metrics.json`
- Verify concurrent writes don't corrupt the file (run two `RecordEvent` calls in goroutines)

### Manual Verification
- Run `devx agent ship` on a project → confirm metrics are written to `~/.devx/metrics.json`
- Run `devx stats` → confirm percentile output renders correctly
- Artificially inject a 65-second build → confirm nudge message appears on stderr

## 8. Self-Review Findings (Gaps Fixed in This Revision)

1. **Missing: JSON mode threading for nudge suppression.** The original plan said `NudgeIfSlow` would respect `--json` but didn't specify *how*. Resolution: the nudge writes to stderr, which `devx agent ship --json` already suppresses at the `cmd` layer. Added explicit note.

2. **Missing: Concurrency safety for metrics file.** Multiple `devx` processes (e.g., multirepo `devx up` + `devx agent ship`) could race on `~/.devx/metrics.json`. Resolution: added `flock`-based file locking.

3. **Missing: Metrics file growth bound.** Without a cap, heavy users would accumulate unbounded JSON. Resolution: FIFO rotation at 1000 entries.

4. **Missing: `devx nuke` interaction.** Should `devx nuke` delete metrics? Resolution: No — metrics are user-level, not project-level. `devx stats --clear` is the explicit reset.

5. **Scope creep: Phase 3 conflated with Phase 1-2.** The original plan mixed container build logic with simple timing instrumentation. Resolution: cleanly separated into three phases with Phase 3 explicitly deferred to a follow-up PR.

6. **Missing: Exact `ship.go` instrumentation point.** The original plan said "wrap the build execution" without specifying where. Resolution: identified the exact lines (157-172 in `RunPreFlight`) and showed the diff.

7. **Missing: `devx up` timing is NOT a build nudge.** The original plan applied the nudge to `devx up` startup time, but startup includes DB provisioning and healthcheck polling — not a "build" the user can optimize with pre-building. Resolution: record the metric but don't nudge.
