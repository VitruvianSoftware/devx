# Genskills

## Overview
Code generator for `apps/cli/vitruvian-devx/internal/genskills` that updates the embedded Cobra command reference in the canonical `devx` skill template and syncs the repository's checked-in `.agents/skills` and `.github/skills` copies from the single source.

## Interface
- `[main.go:main]`

## Verification
- Run: `bazel run //apps/cli/vitruvian-devx/internal/genskills`
- Run: `bazel test //apps/cli/vitruvian-devx/cmd:cmd_test`
