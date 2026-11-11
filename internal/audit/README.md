# Audit

## Overview
Component package providing implementation for `apps/cli/devx/internal/audit`.

## Interface
- `[audit.go:ToolMode]`
- `[audit.go:Tool]`
- `[audit.go:Trivy]`
- `[audit.go:Gitleaks]`
- `[audit.go:Detect]`
- `[audit.go:ErrVMNotRunning]`
- `[audit.go:Run]`
- `[audit.go:InstallPrePushHook]`
- `[audit.go:InstallPreCommitHook]`

## Invariants
- None

## Side Effects
- [audit.go:31]: `"os/exec"`
- [audit.go:121]: `if _, err := exec.LookPath(t.BinaryName); err == nil {`
- [audit.go:147]: `var cmd *exec.Cmd`

## Verification
- Run: `bazel test //apps/cli/devx/internal/audit/...`
