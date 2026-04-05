# Idea 45.4: Pipeline Lifecycle Hooks & Custom Actions Execution

Activate the `before:`/`after:` lifecycle hooks in pipeline stages, and wire up `customActions:` for on-demand execution. Both were parsed-but-not-executed in Idea 45.2.

## User Review Required

> [!IMPORTANT]
> **Custom Actions surface: `devx run` vs new subcommand.** The `devx.yaml.example` documents custom actions as `devx exec <action-name>`, but `devx exec` already exists as a pass-through wrapper for low-level infra tools (`exec.go`). Gemini's plan proposed overloading `devx run` instead. I recommend a **third option**: a dedicated `devx action <name>` subcommand. Rationale:
> - `devx run` has established semantics — it wraps a **single arbitrary host command** with telemetry. Overloading it to also be an action registry lookup creates ambiguous precedence ("did I mean the binary `deploy` or my custom action `deploy`?").
> - `devx exec` is already taken for infra passthrough (`devx exec podman ...`).
> - `devx action <name>` is unambiguous, discoverable via `devx action --list`, and follows the Skaffold `custom` action mental model.
>
> **If you prefer `devx run <action>` instead**, the collision precedence rule (custom action wins over PATH binary) is defensible but should print a visible `ℹ resolved 'deploy' from customActions in devx.yaml` notice so the developer isn't confused.

> [!WARNING]
> **`after:` hooks on stage failure.** Gemini's plan makes `after:` hooks fail-fast alongside the stage — if a stage's main commands fail, `after:` hooks are skipped entirely. This is the simplest model, but it means cleanup hooks (e.g., `after: [["docker", "rm", "-f", "temp-builder"]]`) won't run. Do you want a `cleanup:` hook that always runs (like Go's `defer`), or is fail-fast acceptable?

## Completeness Checklist

- [x] Every file to be modified or created is listed with exact function signatures and line-level changes
- [x] Edge cases addressed: hook failure, skipped stages, empty hooks, missing `devx.yaml`, action name collisions, `go test` interception inside actions
- [x] Error handling: fail-fast with stage-specific error messages; deterministic exit codes preserved
- [x] Environment: all execution on the host via `os/exec`; stdout/stderr inherit TTY routing from parent command

---

## Proposed Changes

### Pipeline Hooks Layer

---

#### [MODIFY] [ship.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/ship/ship.go)

**Struct change** (line 117):
```go
type PipelineStage struct {
    Cmds   [][]string
    Before [][]string  // NEW
    After  [][]string  // NEW
}
```

**Extract a `runStageWithHooks` helper** to eliminate the 4x copy-paste in `runExplicitPipeline`:
```go
// runStageWithHooks executes before → cmds → after for a single pipeline stage.
// Returns nil if the stage pointer is nil or has no commands.
func runStageWithHooks(dir string, stage *PipelineStage, name string, verbose bool) error {
    if stage == nil || len(stage.Cmds) == 0 {
        return nil
    }
    for _, hook := range stage.Before {
        if err := runCmd(dir, hook, verbose); err != nil {
            return fmt.Errorf("%s before hook failed: %w", name, err)
        }
    }
    for _, cmd := range stage.Cmds {
        if err := runCmd(dir, cmd, verbose); err != nil {
            return fmt.Errorf("%s failed: %w", name, err)
        }
    }
    for _, hook := range stage.After {
        if err := runCmd(dir, hook, verbose); err != nil {
            return fmt.Errorf("%s after hook failed: %w", name, err)
        }
    }
    return nil
}
```

**Refactor `runExplicitPipeline`** (lines 140–211) to use the helper. Each stage block collapses from 6 lines to ~5:
```go
// Test
if pipeline.Test != nil && len(pipeline.Test.Cmds) > 0 {
    if err := runStageWithHooks(dir, pipeline.Test, "test", verbose); err != nil {
        return result, err
    }
    result.TestPass = true
} else {
    result.TestSkipped = true
    result.TestPass = true
}
```

This is a pure readability/DRY improvement — the existing 4 copy-pasted blocks for Test/Lint/Build/Verify are already begging for extraction even before hooks. Adding hooks without extracting would create 4 blocks of ~18 lines each.

---

#### [MODIFY] [agent_ship.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/agent_ship.go)

**Update `convertStage`** (lines 311–320) to pass through `Before` and `After`:
```go
func convertStage(s *DevxConfigPipelineStage) *ship.PipelineStage {
    if s == nil {
        return nil
    }
    cmds := s.Cmds()
    if len(cmds) == 0 && len(s.Before) == 0 && len(s.After) == 0 {
        return nil
    }
    return &ship.PipelineStage{
        Cmds:   cmds,
        Before: s.Before,
        After:  s.After,
    }
}
```

> [!NOTE]
> Edge case: a stage with no `command/commands` but with `before:` hooks should still be considered "present" (not nil) so the hooks fire. The nil-guard changes from `len(cmds) == 0` to checking all three fields.

---

### Custom Actions Layer

---

#### [NEW] [cmd/action.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/action.go)

New cobra command `devx action <name>`:
- Loads `devx.yaml` via `resolveConfig()`
- Looks up `cfg.CustomActions[name]`
- If not found: prints available actions and exits with code 1
- If `--list` flag: prints all available action names and exits
- Resolves commands via same `Command`/`Commands` pattern as pipeline stages (add a `Cmds()` method to `DevxConfigCustomAction`)
- Executes each sub-command sequentially with stdout/stderr wired through `io.MultiWriter` (same pattern as `cmd/run.go`)
- **Go test interception**: for each sub-command, checks `telemetry.IsGoTestCmd(cmd)` and routes through `RunGoTestWithTelemetry` if matched (preserving granular test spans)
- Records a single `devx_action` telemetry span for the entire action suite with attributes: `devx.action.name`, `devx.action.exit_code`, `devx.action.command_count`
- Supports `--dry-run`: prints the resolved command list without executing
- Supports `--json`: outputs `{"action": "seed-db", "exit_code": 0, "duration_ms": 1234}`

#### [MODIFY] [cmd/devxconfig.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/devxconfig.go)

Add a `Cmds()` method to `DevxConfigCustomAction` (mirroring `DevxConfigPipelineStage.Cmds()`):
```go
func (ca *DevxConfigCustomAction) Cmds() [][]string {
    if len(ca.Commands) > 0 {
        return ca.Commands
    }
    if len(ca.Command) > 0 {
        return [][]string{ca.Command}
    }
    return nil
}
```

#### [MODIFY] [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example)

Update the scaffolded comment (line 229) from `devx exec <action-name>` to `devx action <action-name>`.

---

### Dogfooding: Update Our Own `devx.yaml`

#### [MODIFY] [devx.yaml](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml)

Add hooks and a custom action to exercise the new code:
```yaml
pipeline:
  test:
    command: ["go", "test", "./..."]
  lint:
    before:
      - ["echo", "▸ Running go vet..."]
    command: ["go", "vet", "./..."]
  build:
    command: ["go", "build", "./..."]
    after:
      - ["echo", "✓ Build artifacts ready"]

customActions:
  ci:
    commands:
      - ["go", "test", "./..."]
      - ["go", "vet", "./..."]
      - ["go", "build", "./..."]
```

---

## Design Decisions

1. **`runStageWithHooks` extraction vs inline**: Gemini's plan described inline hook loops in each stage block. With 4 stages × 3 phases (before/cmd/after), that's 12 loop blocks in one function. Extracting a helper keeps `runExplicitPipeline` at ~40 lines instead of ~80 and makes the hook ordering guarantee (before→cmd→after) testable in isolation.

2. **`devx action` vs overloading `devx run`**: See User Review section above.

3. **Single telemetry span per action**: One `devx_action` span per invocation (not per sub-command). Sub-commands that happen to be `go test` still get their own granular child spans via the existing `RunGoTestWithTelemetry` interception.

4. **Nil-guard widening on `convertStage`**: A stage with only `before:` hooks (no main commands) is a valid use case (e.g., a "setup" stage that just runs migrations). The nil-guard must check all three fields.

## Gap Analysis

| Gap | Severity | Remediation |
|-----|----------|-------------|
| `devx.yaml.example` documents custom actions as `devx exec <action-name>` but `devx exec` is already taken for infra passthrough | Medium | Update comment to `devx action <action-name>` |
| `DevxConfigCustomAction` has no `Cmds()` helper — unlike `DevxConfigPipelineStage` which has one | Low | Add the method for consistency |
| `runExplicitPipeline` has 4 copy-pasted stage blocks that will double in size with hooks | Medium | Extract `runStageWithHooks` |
| `go test` commands inside custom actions won't get granular test interception | Medium | Check `IsGoTestCmd` per sub-command in the action loop |
| `FEATURES.md` Idea 45.2 entry says hooks are "parsed/validated in 45.2, executed in 45.3" — but 45.3 shipped without executing them | Low | Update FEATURES.md to attribute hook execution to 45.4 |

## Error Handling Strategy

- **Pipeline `before:` failure**: Aborts immediately. Main `Cmds` and `after:` hooks do NOT run. Error message: `"test before hook failed: exit status 1"`. Preflight result records `TestPass = false`.
- **Pipeline `after:` failure**: Aborts immediately. Error propagates as `"build after hook failed: ..."`. The stage's main commands already succeeded, so telemetry for the build duration is still recorded.
- **Custom action sub-command failure**: Loop breaks, exit code from the failing sub-process propagates via `os.Exit(exitCode)`. Remaining sub-commands are skipped.
- **Missing `devx.yaml`**: Custom actions gracefully degrade — `resolveConfig` returns an error, action lookup returns "no customActions defined", exit 1.

## Documentation Updates

After verification passes:
- `FEATURES.md`: Add Idea 45.4 entry documenting hook execution and custom actions
- `docs/guide/pipeline.md`: Add "Lifecycle Hooks" section with before/after examples; add "Custom Actions" section with `devx action` usage
- `devx.yaml.example`: Update scaffolded comments (exec → action, uncomment examples)
- `CONTRIBUTING.md`: Update project structure to include `action.go`

## Verification Plan

### Automated Tests
```bash
go test -race ./internal/ship/...   # Hook ordering and fail-fast
go test -race ./cmd/...             # Action resolution and config parsing
go vet ./...                        # Vet
golangci-lint run ./...             # Linter
go build ./...                      # Compilation
```

### Edge Case Scenarios
1. **Hook failure aborts stage**: Add `before: [["false"]]` to lint stage, run `devx agent ship --skip-preflight=false -m test`, confirm lint stage fails before `go vet` runs
2. **Empty hooks are no-ops**: Stage with `before: []` behaves identically to stage without `before:` key
3. **Custom action not found**: `devx action nonexistent` prints available actions and exits 1
4. **Custom action with `go test`**: `devx action ci` produces granular test spans in Grafana
5. **Dry-run for actions**: `devx action ci --dry-run` prints the 3 resolved commands without executing
6. **No `devx.yaml`**: `devx action foo` in a directory without `devx.yaml` prints a clear error, not a panic

### Dogfood Verification
- Run `devx agent ship -m "feat(pipeline): implement lifecycle hooks and custom actions"` with the updated `devx.yaml` — the `before:` echo on lint should print visibly in Phase 1 output
- Run `devx action ci` and verify all 3 commands execute sequentially with a single telemetry span

### Post-Verification
- Update all documentation files listed above
- Ship via `devx agent ship`
