# Testutil

## Overview
Component package providing implementation for `apps/cli/devx/internal/testutil`.

## Interface
- `[fake_runtime.go:FakeRuntime]`
- `[fake_runtime.go:SetupFakeRuntime]`
- `[fake_runtime.go:Requests]`

## Invariants
- None

## Side Effects
- [fake_runtime.go:64]: `if err := os.WriteFile(stubPath, []byte(stubPython), 0o755); err != nil {`

## Verification
- Run: `bazel test //apps/cli/devx/internal/testutil/...`
