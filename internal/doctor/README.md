# Doctor

## Overview
Component package providing implementation for `apps/cli/devx/internal/doctor`.

## Interface
- `[auth.go:AuthStep]`
- `[auth.go:AuthPlan]`
- `[auth.go:RunAuthStep]`
- `[check.go:ToolStatus]`
- `[check.go:CredentialStatus]`
- `[check.go:SystemInfo]`
- `[check.go:AIProviderStatus]`
- `[check.go:AILandscape]`
- `[check.go:Report]`
- `[check.go:DetectSystem]`
- `[check.go:CheckTools]`
- `[check.go:CheckCredentials]`
- `[check.go:RunFullAudit]`
- `[check.go:CheckAILandscape]`
- `[install.go:InstallPlan]`
- `[install.go:InstallStep]`
- `[install.go:PlanInstall]`
- `[install.go:ExecuteInstall]`

## Invariants
- None

## Side Effects
- [auth.go:27]: `"os/exec"`
- [auth.go:94]: `out, err := exec.Command("gh", "auth", "status").CombinedOutput()`
- [auth.go:147]: `cmd := exec.Command("cloudflared", "login")`
- [check.go:27]: `"os/exec"`
- [check.go:252]: `if path, err := exec.LookPath("brew"); err == nil {`
- [check.go:254]: `if out, err := exec.Command(path, "--version").Output(); err == nil {`
- [install.go:26]: `"os/exec"`
- [install.go:151]: `// stub for the real exec-backed runner.`
- [install.go:158]: `c := exec.Command(parts[0], parts[1:]...)`

## Verification
- Run: `bazel test //apps/cli/devx/internal/doctor/...`
