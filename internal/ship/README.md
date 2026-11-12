# Ship

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/ship`.

## Interface
- `[hook.go:PreCommitHookContent]`
- `[hook.go:PrePushHookContent]`
- `[hook.go:InstallPreCommitHook]`
- `[hook.go:InstallPrePushHook]`
- `[hook.go:IsPreCommitHookInstalled]`
- `[hook.go:IsPrePushHookInstalled]`
- `[ship.go:ErrNoStack]`
- `[ship.go:Result]`
- `[ship.go:PreFlightResult]`
- `[ship.go:Options]`
- `[ship.go:DetectStack]`
- `[ship.go:DetectAllStacks]`
- `[ship.go:PipelineStage]`
- `[ship.go:PipelineConfig]`
- `[ship.go:RunPreFlight]`
- `[ship.go:HasUnpushedCommits]`
- `[ship.go:LatestCommitMessage]`
- `[ship.go:GitPush]`
- `[ship.go:CreatePR]`
- `[ship.go:SplitCommitMessage]`

## Invariants
- None

## Side Effects
- [hook.go:131]: `if err := os.WriteFile(hookPath, []byte(PreCommitHookContent), 0o755); err !=...`
- [hook.go:158]: `if err := os.WriteFile(hookPath, []byte(PrePushHookContent), 0o755); err != n...`
- [ship.go:34]: `"os/exec"`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/ship/...`
