# Sync

## Overview
Component package providing implementation for `apps/cli/devx/internal/sync`.

## Interface
- `[daemon.go:Session]`
- `[daemon.go:DefaultIgnores]`
- `[daemon.go:EnsureDaemon]`
- `[daemon.go:CreateSession]`
- `[daemon.go:TerminateSession]`
- `[daemon.go:TerminateAll]`
- `[daemon.go:ListSessions]`
- `[daemon.go:IsInstalled]`

## Invariants
- None

## Side Effects
- [daemon.go:28]: `"os/exec"`
- [daemon.go:62]: `cmd := exec.Command("mutagen", "daemon", "start")`
- [daemon.go:95]: `cmd := exec.Command("mutagen", args...)`

## Verification
- Run: `bazel test //apps/cli/devx/internal/sync/...`
