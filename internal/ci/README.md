# Ci

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/ci`.

## Interface
- `[executor.go:RunResult]`
- `[executor.go:StepResult]`
- `[executor.go:ExecuteConfig]`
- `[executor.go:Execute]`
- `[parser.go:Workflow]`
- `[parser.go:Job]`
- `[parser.go:Strategy]`
- `[parser.go:MatrixDef]`
- `[parser.go:UnmarshalYAML]`
- `[parser.go:Service]`
- `[parser.go:Step]`
- `[parser.go:StringOrSlice]`
- `[parser.go:UnmarshalYAML]`
- `[parser.go:ExpandedJob]`
- `[parser.go:ParseWorkflow]`
- `[parser.go:DiscoverWorkflows]`
- `[parser.go:ExpandMatrix]`
- `[parser.go:ResolveJobDAG]`
- `[template.go:TemplateContext]`
- `[template.go:NewTemplateContext]`

## Invariants
- None

## Side Effects
- [executor.go:28]: `"os/exec"`
- [executor.go:207]: `cmd := exec.Command(cfg.Runtime, createArgs...)`
- [executor.go:214]: `cmd = exec.Command(cfg.Runtime, createArgs...)`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/ci/...`
