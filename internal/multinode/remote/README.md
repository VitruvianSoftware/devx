# Remote

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/multinode/remote`.

## Interface
- `[remote.go:DefaultMaxRetries]`
- `[remote.go:Runner]`
- `[remote.go:NewRunner]`
- `[remote.go:NewRunnerWithOpts]`
- `[remote.go:Run]`
- `[remote.go:RunShell]`
- `[remote.go:LimaShell]`
- `[remote.go:LimaShellSudo]`
- `[remote.go:RunSudo]`

## Invariants
- None

## Side Effects
- [remote.go:32]: `"os/exec"`
- [remote.go:147]: `cmd := exec.CommandContext(ctx, "ssh", args...)`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/multinode/remote/...`
