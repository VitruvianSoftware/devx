# Exposure

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/exposure`.

## Interface
- `[domain.go:GenerateDomain]`
- `[store.go:Entry]`
- `[store.go:Save]`
- `[store.go:RemoveAll]`
- `[store.go:RemoveByName]`
- `[store.go:LoadAll]`
- `[store.go:LookupPort]`

## Invariants
- None

## Side Effects
- [store.go:138]: `return os.WriteFile(path, data, 0o644)`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/exposure/...`
