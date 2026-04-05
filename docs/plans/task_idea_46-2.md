# Idea 46.2a: Inbound Traffic Interception — Task Tracker

## Core Infrastructure
- [x] Add `hashicorp/yamux` dependency to `go.mod`
- [x] Add `google/uuid` dependency to `go.mod`
- [x] Add new error codes to `internal/devxerr/error.go` (65–72)
- [x] Extend `DevxConfigBridge` schema in `cmd/devxconfig.go` with `AgentImage` + `Intercepts`

## Internal Packages
- [x] Create `internal/bridge/intercept.go` — Service inspection, validation, selector swap/restore
- [x] Create `internal/bridge/agent.go` — Agent Job lifecycle (deploy, remove, dynamic Pod spec)
- [x] Create `internal/bridge/tunnel.go` — Yamux tunnel management (CLI side)
- [x] Extend `internal/bridge/session.go` — Add `InterceptEntry` to `Session`

## CLI Commands
- [x] Create `cmd/bridge_intercept.go` — `devx bridge intercept` subcommand
- [x] Create `cmd/bridge_rbac.go` — `devx bridge rbac` helper
- [x] Modify `cmd/bridge.go` — Register intercept + rbac subcommands
- [x] Modify `cmd/bridge_disconnect.go` — Extend teardown for intercepts
- [x] Modify `cmd/bridge_status.go` — Show intercept sessions

## Agent Binary (Separate Module)
- [x] Create `internal/bridge/agent/` directory with `go.mod`
- [x] Create agent `main.go` — Yamux server, dynamic port listeners, self-healing
- [x] Create `Dockerfile` for agent image
- [x] Create `.github/workflows/bridge-agent.yml` for CI

## Tests
- [x] Create `internal/bridge/intercept_test.go` (7 tests)
- [x] Create `internal/bridge/agent_test.go` (7 tests)
- [x] Create `internal/bridge/tunnel_test.go` (3 tests)

## Documentation
- [x] Update `docs/guide/bridge.md` — Full intercept section with RBAC, safety, service mesh
- [x] Update `IDEAS.md` — Mark 46.2 as Implemented
- [x] Update `FEATURES.md` — Add 46.2 feature entry
- [x] Update `devx.yaml.example` — Intercept config example
- [x] Update `.agent/skills/devx/SKILL.md` — Intercept commands, exit codes, architecture
- [x] Sync SKILL.md to all 5 template locations
- [x] Update `docs/guide/architecture.md` — Bridge layer reflects 46.2 as shipped

## Verification
- [x] `devx action ci` — All stages green (vet, test, build) ✅
