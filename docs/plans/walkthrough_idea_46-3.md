# Walkthrough: Idea 46.3 — Full Hybrid Topology (`runtime: bridge`)

## Summary

Implemented declarative bridge services in `devx up`. Services with `runtime: bridge` and either `bridge_target` (outbound) or `bridge_intercept` (inbound) participate in the DAG orchestrator with correct dependency ordering, bridge-native health checks, and unified lifecycle management.

## Files Changed

### New Files
| File | Purpose |
|------|---------|
| [bridge_node.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/orchestrator/bridge_node.go) | DAG bridge handler: `startBridgeConnect`, `startBridgeIntercept`, `waitForBridgeHealthy`, `BridgeNodeState.Cleanup`, env file generation |

### Modified Source Files
| File | Changes |
|------|---------|
| [devxconfig.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/devxconfig.go) | Added `DevxConfigServiceBridgeTarget`, `DevxConfigServiceBridgeIntercept` types; extended `DevxConfigService`; added `validateBridgeServices()`; extended `mergeProfile()` |
| [dag.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/orchestrator/dag.go) | Added `RuntimeBridge`, `BridgeMode`, `BridgeNodeConfig`; refactored dispatch guard to `switch n.Runtime`; extended cleanup for bridge teardown; added bridge-native health path |
| [session.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/bridge/session.go) | Added `Origin` field to `SessionEntry` and `InterceptEntry` |
| [bridge_disconnect.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_disconnect.go) | Filters DAG-managed entries with `Origin: "dag"`, shows warning |
| [up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go) | Wired `runtime: bridge` into service registration with `BridgeNodeConfig` construction; added post-DAG `bridge.env` generation |

### Documentation
| File | Changes |
|------|---------|
| [bridge.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/bridge.md) | New "Hybrid Topology" section with YAML examples, behavior table, session isolation |
| [orchestration.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/orchestration.md) | Added `bridge` runtime to the runtimes table |
| [architecture.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/architecture.md) | Marked 46.3 as shipped |
| [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example) | Full bridge service examples (connect + intercept + dependent local service) |
| [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md) | 46.3 feature entry |
| [IDEAS.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/IDEAS.md) | Status → Implemented |
| [SKILL.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/.agent/skills/devx/SKILL.md) | runtime: bridge awareness |

## Key Architectural Decisions

### 1. Dispatch Guard Refactor
The `Execute()` goroutine originally guarded on `len(n.Command) > 0`. Bridge nodes have no `Command` (they use `BridgeConfig`), so they'd be silently skipped. Refactored to `switch n.Runtime` with `RuntimeBridge` dispatching to `startBridgeNode()`.

### 2. Connect vs Intercept Blocking
- **Connect**: `pf.Start()` blocks forever → spawned in goroutine. Readiness polled via `pf.State()`.
- **Intercept**: Finite setup (~10-30s) runs synchronously. `startBridgeIntercept` returning `nil` IS the readiness signal.

### 3. No TCP Healthcheck for Intercept
A naive `checkTCP("localhost:<port>")` for intercept nodes would give false positives because the local dev app owns that port, not the bridge. Intercept nodes skip the healthcheck phase entirely — their setup completion is the readiness guarantee.

### 4. Session Ownership
`Origin: "dag"` field prevents `devx bridge disconnect` from tearing down DAG-managed bridges while `devx up` is running.

## Verification

| Check | Result |
|-------|--------|
| `go build ./...` | ✅ Clean |
| `go test ./... -short` | ✅ All pass (including existing orchestrator tests) |
| `go vet ./...` | ✅ No issues |
