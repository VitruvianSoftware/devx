# Devxerr

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/devxerr`.

## Interface
- `[error.go:DevxError]`
- `[error.go:Error]`
- `[error.go:Unwrap]`
- `[error.go:New]`
- `[recovery.go:RecoverGcloudAuth]`

## Invariants
- None

## Side Effects
- [error.go:70]: `// Natural Language DB Query Errors (Idea 60)`
- [error.go:71]: `CodeDBAskNoAI        = 91 // No AI provider for natural language query`
- [recovery.go:26]: `"os/exec"`
- [recovery.go:61]: `cmd := exec.Command("gcloud", "auth", "login")`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/devxerr/...`
