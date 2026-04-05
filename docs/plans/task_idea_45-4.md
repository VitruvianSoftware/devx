# Idea 45.4: Pipeline Hooks & Custom Actions — Task List

## Pipeline Hooks
- [x] Add `Before`/`After` fields to `ship.PipelineStage` struct
- [x] Extract `runStageWithHooks` helper in `ship.go`
- [x] Refactor `runExplicitPipeline` to use the helper for all 4 stages
- [x] Update `convertStage` in `agent_ship.go` with widened nil-guard
- [x] Unit test: all existing tests pass (no regressions)

## Custom Actions
- [x] Add `Cmds()` method to `DevxConfigCustomAction`
- [x] Create `cmd/action.go` with `devx action <name>` subcommand
- [x] Implement `--list`, `--dry-run`, `--json` flags
- [x] Go test interception inside action sub-commands
- [x] Single `devx_action` telemetry span per invocation

## Dogfooding
- [x] Update `devx.yaml` with hooks and a `ci` custom action
- [x] Update `devx.yaml.example` comments (exec → action)

## Documentation
- [x] `FEATURES.md`: Add Idea 45.4 entry
- [x] `docs/guide/pipeline.md`: Lifecycle hooks + custom actions sections
- [ ] `CONTRIBUTING.md`: Add `action.go` to project structure (deferred — minor)

## Verification
- [x] `go test -race ./...` — 17 packages pass
- [x] `go vet ./...` — clean
- [x] `go build ./...` — clean
- [x] Manual: `devx action --list` — shows `ci (3 commands)`
- [x] Manual: `devx action ci --dry-run` — prints 3 commands
- [x] Manual: `devx action nonexistent` — shows available actions, exit 1
- [x] Manual: `devx agent ship` — preflight passes with hooks
