# Preview

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/preview`.

## Interface
- `[sandbox.go:Sandbox]`
- `[sandbox.go:New]`
- `[sandbox.go:Setup]`
- `[sandbox.go:Run]`
- `[sandbox.go:Teardown]`
- `[sandbox.go:DryRun]`
- `[sandbox.go:JSON]`

## Invariants
- [sandbox.go:244]: `// This works even if devx.yaml parsing would require importing the config pa...`

## Side Effects
- [sandbox.go:31]: `"os/exec"`
- [sandbox.go:64]: `if _, err := exec.LookPath("gh"); err != nil {`
- [sandbox.go:69]: `if err := exec.Command("gh", "auth", "status").Run(); err != nil {`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/preview/...`
