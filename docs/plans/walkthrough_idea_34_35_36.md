# Walkthrough: P0 Killer Sprint (Ideas 34, 35, 36)

## Summary

This sprint transforms `devx up` from a linear infrastructure provisioner into a **DAG-based service orchestrator** with automatic port conflict resolution and inline crash diagnostics.

---

## Changes Made

### Idea 36: Automatic Port Conflict Resolution

#### [NEW] [ports.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/network/ports.go)
Core port utilities:
- `CheckPortAvailable(port)` — probes `127.0.0.1:<port>` via `net.Listen`
- `GetFreePort()` — binds `:0` to let the OS assign a free port
- `ResolvePort(desired)` — checks availability, auto-shifts if occupied, returns a high-contrast warning string

#### [NEW] [ports_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/network/ports_test.go)
5 unit tests covering free/occupied detection, auto-shifting, and warning generation.

#### [MODIFY] [up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go)
- Database ports and tunnel ports are now resolved through `network.ResolvePort()` before spawning
- When a shift occurs, a developer-facing warning is emitted to stderr explaining the impact on hardcoded configs

---

### Idea 35: Context-Aware Log-Tailing on Crash

#### [NEW] [crashlog.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/logs/crashlog.go)
Two helpers:
- `TailContainerCrashLogs(runtime, name, lines)` — runs `podman logs --tail N` and renders in a lipgloss error box
- `TailHostCrashLogs(serviceName, lines)` — reads last N lines from `~/.devx/logs/<name>.log`

Both render through `renderCrashBox()` using lipgloss red-bordered rounded boxes for maximum crash visibility.

#### [MODIFY] [up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go)
- Database spawn failures now immediately dump crash logs inline
- Service orchestration failures (via DAG) also trigger crash log tailing

---

### Idea 34: Service Dependency Graphs (DAG Orchestration)

#### [NEW] [dag.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/orchestrator/dag.go)
Full DAG-based orchestrator:
- `NewDAG()` / `AddNode()` / `Validate()` — graph construction and validation
- `TopologicalSort()` — Kahn's algorithm producing execution tiers for parallel startup
- `Execute(ctx)` — tiered parallel execution with health check gating between tiers
- `waitForHealthy()` — polls HTTP endpoints and TCP ports with configurable interval/timeout/retries
- `startHostProcess()` — launches native host processes with log routing to `~/.devx/logs/`

#### [NEW] [dag_test.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/orchestrator/dag_test.go)
7 unit tests covering:
- Linear dependency chains (A → B → C)
- Parallel independent nodes
- Diamond dependencies (DB → API+Worker → Gateway)
- Cycle detection
- Missing dependency validation
- Duplicate node detection

#### [MODIFY] [up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go)
Major refactor:
- Added `DevxConfigService`, `DevxConfigDependsOn`, `DevxConfigHealthcheck` YAML structs
- Added `Services []DevxConfigService` to `DevxConfig`
- After database provisioning and tunnel setup, constructs a DAG from all services
- Registers database nodes as dependencies (for `depends_on` resolution)
- Executes the DAG in topological order with health check gating
- Signal handling (SIGINT/SIGTERM) for graceful shutdown of services when no tunnels are defined

#### [MODIFY] [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example)
Added comprehensive `services:` section demonstrating:
- Multi-runtime support (`host`, `container`, `kubernetes`, `cloud`)
- `depends_on` with `service_healthy` conditions
- Health checks (HTTP and TCP)
- Custom environment variables
- A realistic 3-service topology: `api` → `worker` → `web`

---

## Test Results

| Package | Tests | Result |
|---------|-------|--------|
| `internal/network` | 5 | ✅ PASS |
| `internal/orchestrator` | 7 | ✅ PASS |
| `cmd` (existing) | 4 | ✅ PASS |
| All other packages | — | ✅ No regressions |

**Full build:** `go build ./...` — clean, zero errors.

---

## Remaining Work

| Task | Status |
|------|--------|
| Manual verification with live VM | ⏳ Requires `devx vm init` |
| Screenshots for documentation | ⏳ Blocked on above |
| `docs/guide/orchestration.md` | ⏳ Will create after screenshots |
| `README.md` marketing language update | ⏳ Will create after docs |
