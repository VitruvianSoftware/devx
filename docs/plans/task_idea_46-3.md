# Idea 46.3: Full Hybrid Topology — Task Tracker

## Phase 1: Schema & Core Types
- [x] Add `DevxConfigServiceBridgeTarget` and `DevxConfigServiceBridgeIntercept` to `devxconfig.go`
- [x] Extend `DevxConfigService` with `BridgeTarget` and `BridgeIntercept` fields
- [x] Add `validateBridgeServices()` helper
- [x] Extend `mergeProfile()` for bridge-specific fields

## Phase 2: DAG Orchestrator
- [x] Add `RuntimeBridge` constant to `dag.go`
- [x] Add `BridgeMode`, `BridgeNodeConfig`, `bridgeState` fields to `Node`
- [x] Refactor `Execute()` dispatch guard from `len(n.Command) > 0` to `switch n.Runtime`
- [x] Extend cleanup function for bridge teardown
- [x] Add bridge-native health path to `waitForHealthy()` for connect nodes

## Phase 3: Bridge Node Handler
- [x] Create `internal/orchestrator/bridge_node.go`
- [x] Implement `startBridgeConnect()` with goroutine-spawned `pf.Start()`
- [x] Implement `startBridgeIntercept()` with synchronous finite setup

## Phase 4: Session Ownership
- [x] Add `Origin` field to `SessionEntry` and `InterceptEntry` in `session.go`
- [x] Update `bridge_disconnect.go` to skip DAG-managed entries

## Phase 5: Up Command Integration
- [x] Wire `runtime: bridge` into service registration loop in `up.go`
- [x] Add post-DAG `bridge.GenerateEnvFile()` call
- [x] Add `validateBridgeServices()` call after config resolution

## Phase 6: Tests
- [x] Extend `cmd/devxconfig_test.go` with bridge validation tests — 10 tests added (valid target/intercept, missing bridge section, neither/both, missing fields, invalid mode, profile merge, non-bridge ignored)
- [x] Extend `internal/orchestrator/dag_test.go` with RuntimeBridge nodes — 5 tests added (bridge-before-service, intercept-depends-on-local, parallel bridges, env vars, toEnvName)
- [x] Create `internal/orchestrator/bridge_node_test.go` — Test logic written for error cases, bridge state cleanup, and JSON manifest generation without needing cluster mocks.

## Phase 7: Documentation
- [x] Update `docs/guide/bridge.md` — hybrid topology section
- [x] Update `docs/guide/orchestration.md` — runtime table
- [x] Update `docs/guide/architecture.md` — 46.3 shipped
- [x] Update `devx.yaml.example` — bridge service examples
- [x] Update `FEATURES.md` — 46.3 entry
- [x] Update `IDEAS.md` — mark 46.3 implemented
- [x] Update `SKILL.md` — ALL 6 locations (`.agent`, `.github`, `templates/.agent`, `templates/.claude`, `templates/.cursor`, `templates/.github`)
- [x] Update `README.md` — hybrid bridge feature bullet

## Phase 8: Verification
- [x] `go build ./...` — passes
- [x] `go test ./... -short` — all existing tests pass
- [x] `devx action ci` — ran via `go run main.go action ci` locally (mandated SOP) and it succeeded cleanly.
