# Nuke

## Overview
Component package providing implementation for `apps/cli/devx/internal/nuke`.

## Interface
- `[nuke.go:Item]`
- `[nuke.go:Manifest]`
- `[nuke.go:Collect]`
- `[nuke.go:Execute]`
- `[nuke.go:FormatBytes]`

## Invariants
- None

## Side Effects
- [nuke.go:29]: `"os/exec"`
- [nuke.go:172]: `out, err := runtime.Exec(`
- [nuke.go:193]: `out, err = runtime.Exec(`

## Verification
- Run: `bazel test //apps/cli/devx/internal/nuke/...`
