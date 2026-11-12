# Secrets

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/secrets`.

## Interface
- `[loader.go:Secrets]`
- `[loader.go:NonInteractive]`
- `[loader.go:Load]`
- `[loader.go:Save]`
- `[loader.go:Rotate]`

## Invariants
- None

## Side Effects
- [loader.go:71]: `return os.WriteFile(envFile, []byte(content), 0o600)`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/secrets/...`
