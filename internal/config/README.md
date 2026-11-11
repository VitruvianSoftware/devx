# Config

## Overview
Component package providing implementation for `apps/cli/devx/internal/config`.

## Interface
- `[config.go:Config]`
- `[config.go:New]`
- `[config.go:Validate]`
- `[findconfig.go:ErrConfigNotFound]`
- `[findconfig.go:FindProjectConfig]`
- `[local.go:LocalConfig]`
- `[local.go:LocalConfigDir]`
- `[local.go:LocalConfigPath]`
- `[local.go:LoadLocal]`
- `[local.go:SaveLocal]`

## Invariants
- None

## Side Effects
- [local.go:84]: `return os.WriteFile(LocalConfigPath(), data, 0o644)`

## Verification
- Run: `bazel test //apps/cli/devx/internal/config/...`
