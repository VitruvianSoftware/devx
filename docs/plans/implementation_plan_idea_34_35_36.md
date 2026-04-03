# P0 Killer Sprint: devx Multi-Service Orchestration

This implementation plan covers the delivery of Ideas 34, 35, and 36, focused on solidifying `devx` as a production-grade local orchestration platform.

## Proposed Changes

### 1. `devx.yaml` Config Abstractions & Core Pipeline (Idea 34)

We will expand `devx.yaml` configurations to support native application lifecycle execution, which is a hard prerequisite for cross-service dependency mapping. This mirrors the familiar setup in `skaffold.dev` and `docker compose`, dramatically lowering the learning curve and developer friction.

#### [MODIFY] `internal/config/config.go`
Introduce new YAML abstractions to map the service boot sequence:
- `DevxConfigService`: Represents a multi-mode application definition.
  - `name`: Target name.
  - `runtime`: Supports `host`, `container`, `kubernetes`, or `cloud` (defaulting to `host` for MVP) to maintain ultimate flexible execution options across ecosystems.
  - `command`: How to execute it.
  - `depends_on`: Which services/databases need to be healthy.
  - `healthcheck`: Configuration mapping an HTTP or TCP check, retries, and intervals.

#### [MODIFY] `cmd/up.go`
- Instead of linearly executing `db spawn` and tunnels, we will construct a **Directed Acyclic Graph (DAG)** of all requested components: `databases`, `mocks`, and `services`.
- Ensure components boot in parallel where possible, polling dependencies using the defined `healthcheck` before booting trailing applications.

### 2. Context-Aware Log-Tailing on Crash (Idea 35)

When a complex startup sequence fails, we need to bring the error inline rather than expecting the user to switch context continuously to `devx logs`.

#### [MODIFY] `cmd/up.go` and `cmd/run.go`
- Improve the internal error catching mechanism when a scheduled process or infrastructure container dies.
- Implement an inline log-tailer: 
  - For containers (`db`, `mock`): trigger `podman logs --tail 50 <name>` 
  - For host-native applications (`services`): Read the last 50 lines directly from `~/.devx/logs/<name>.log`.
- Dump this context directly to standard output wrapped in a Bubble Tea alert box for maximum visibility, prior to exiting with the generic error payload.

### 3. Automatic Port Conflict Resolution (Idea 36)

To completely eliminate the `EADDRINUSE` friction that interrupts flow states, `devx` will intelligently manage and auto-shift defined ports.

#### [MODIFY] `internal/network/ports.go`
- Implement robust free-port selection (`GetFreePort`) using `net.Listen("tcp", "127.0.0.1:0")`.
- Implement `CheckPortAvailable(port int)` scanner helper.

#### [MODIFY] `cmd/up.go`
- Add a startup hook prior to booting a database, mock, or service. If the hardcoded port (e.g., `:8080`) is busy, dynamically substitute it via `GetFreePort()`.
- Rewrite the `targetPort` configurations for the `cloudflared` exposure tunnel on the fly so the public URL still correctly routes to the new random port smoothly.
- **Developer Warning System:** When a shift occurs, emit a high-contrast Bubble Tea warning alerting the developer that the port was shifted, explaining the potential impact that hardcoded static configs (e.g., expecting `localhost:5432`) may break, and explicitly detailing to use injected `$DB_PORT` or `$PORT` variables instead.
- Inject overridden runtime ports directly into the `devx` environment so the local apps bind dynamically.

### 4. Product Language & Marketing Updates

#### [MODIFY] `README.md` and `docs/guide/orchestration.md` (New)
- We will actively update the product marketing text defining `devx` orchestration as a seamless blend of Docker Compose/Skaffold behaviors, designed to drop learning curves to zero.

## Verification & Documentation Plan

As part of the closing task, I will physically execute these commands in the terminal using the `run_command` tool. I will use the subagent Browser capabilities (if applicable) or generate visual artifacts/screenshots of the Terminal outcomes.

> [!IMPORTANT]
> The screenshots natively captured during my automated manual verification passes will be embedded directly into the official `devx` markdown documentation to visually prove the capabilities.

### Automated Tests
- Append integration test to `cmd/up_test.go` combining multiple DAG dependencies, testing that a dependent service completely fails (or delays) if a database health check command yields negative status codes manually.
- Add test coverage asserting correct `tunnel` rewrites for shifted ports and the emission of the Port Override warning log.

### Manual Verification Path
1. Spin up an explicit blocking application running heavily on port `:3000`.
2. Start `devx up` containing a `devx.yaml` tunnel binding to `:3000`. Assert the startup sequence doesn't fail, safely shifts the port, warns the user, and exposes seamlessly via Cloudflare. Take terminal screenshots of the Warning and the Output.
3. Inject a syntax failure randomly inside a mapped service build step, and assert `devx up` successfully catches, prints the last 50 lines, and cancels dependent pipelines. Take terminal screenshots for debugging documentation.
