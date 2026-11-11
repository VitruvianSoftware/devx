# Cron

## Overview
Component package providing implementation for `apps/cli/devx/internal/cron`.

## Interface
- `[cron.go:Job]`
- `[cron.go:Validate]`
- `[cron.go:ExecuteConfig]`
- `[cron.go:Result]`
- `[cron.go:Execute]`
- `[cron.go:ParseTimeout]`
- `[cron.go:SortedNames]`
- `[cron.go:FindByName]`

## Invariants
- [cron.go:75]: `// for tests so we can assert on captured output.`

## Side Effects
- [cron.go:34]: `"os/exec"`
- [cron.go:73]: `WorkDir string            // working directory for the subprocess; "" means i...`
- [cron.go:111]: `cmd := exec.CommandContext(runCtx, cfg.Job.Command[0], cfg.Job.Command[1:]...)`

## Verification
- Run: `bazel test //apps/cli/devx/internal/cron/...`
