# Tui

## Overview
Component package providing implementation for `apps/cli/devx/internal/tui`.

## Interface
- `[setup.go:StepStatus]`
- `[setup.go:Step]`
- `[setup.go:ProgressMsg]`
- `[setup.go:SetupResult]`
- `[setup.go:SetupCompleteMsg]`
- `[setup.go:AuthURLMsg]`
- `[setup.go:SetupModel]`
- `[setup.go:NewSetupModel]`
- `[setup.go:Init]`
- `[setup.go:Update]`
- `[setup.go:View]`
- `[setup.go:SetResult]`
- `[setup.go:IsDone]`
- `[setup.go:IsFailed]`

## Invariants
- None

## Side Effects
- None

## Verification
- Run: `bazel test //apps/cli/devx/internal/tui/...`
