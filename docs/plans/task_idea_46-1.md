# Idea 46.1: Outbound Bridge — Implementation Task List

## Foundation
- [x] Add bridge exit codes to `internal/devxerr/error.go`
- [x] Add `DevxConfigBridge*` schema types to `cmd/devxconfig.go`
- [x] Add bridge merging to `mergeProfile()` and `loadAndResolve()`

## Internal Bridge Package (`internal/bridge/`)
- [x] Create `internal/bridge/kube.go` — kubeconfig/context validation
- [x] Create `internal/bridge/portforward.go` — kubectl port-forward lifecycle
- [x] Create `internal/bridge/session.go` — session state persistence
- [x] Create `internal/bridge/env.go` — bridge env var generation

## CLI Commands
- [x] Create `cmd/bridge.go` — parent subcommand group
- [x] Create `cmd/bridge_connect.go` — core connect command
- [x] Create `cmd/bridge_status.go` — session status display
- [x] Create `cmd/bridge_disconnect.go` — teardown command
- [x] Register `bridgeCmd` in `cmd/root.go`

## Integration with Existing Commands
- [x] Modify `cmd/shell.go` — source bridge.env when active
- [x] Modify `cmd/doctor.go` — add kubectl conditional prerequisite
- [x] Modify `cmd/nuke.go` — include bridge cleanup
- [ ] Modify `cmd/map.go` — render remote bridged services (deferred to Idea 46.1 follow-up)

## Tests
- [x] Create `internal/bridge/portforward_test.go` (5 tests PASS)
- [x] Create `internal/bridge/session_test.go` (5 tests PASS)

## Build Verification
- [x] `go vet ./...` — clean
- [x] `go build ./...` — clean
- [x] `go test ./... -count=1` — all packages PASS

## Documentation
- [x] Create `docs/guide/bridge.md`
- [x] Update `docs/.vitepress/config.mjs` sidebar
- [x] Update `devx.yaml.example` with bridge section
- [x] Update `FEATURES.md` with Idea 46.1 entry
- [x] Update `IDEAS.md` — mark Idea 46.1 shipped, phases documented
- [x] Update `PRODUCT_ANALYSIS.md` — "Client-Driven Architecture"
- [x] Update `docs/guide/architecture.md` — bridge layer section
- [x] Update `.agent/skills/devx/SKILL.md` — bridge section (#11)
