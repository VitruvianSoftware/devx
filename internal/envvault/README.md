# Envvault

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/envvault`.

## Interface
- `[vault.go:PullAll]`
- `[vault.go:PushAll]`

## Invariants
- None

## Side Effects
- [vault.go:28]: `"os/exec"`
- [vault.go:107]: `if _, err := exec.LookPath("op"); err != nil {`
- [vault.go:116]: `cmd := exec.Command("op", "read", ref)`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/envvault/...`
