# Prereqs

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/prereqs`.

## Interface
- `[check.go:Result]`
- `[check.go:Check]`
- `[check.go:AllPassed]`
- `[check.go:Summary]`
- `[check.go:MissingList]`

## Invariants
- None

## Side Effects
- [check.go:25]: `"os/exec"`
- [check.go:42]: `out, err := exec.Command(tool, "--version").Output()`
- [check.go:45]: `out, err = exec.Command(tool, "version").Output()`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/prereqs/...`
