# Agent

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/agent`.

## Interface
- `[embed.go:EmbedFS]`
- `[embed.go:AgentBasePaths]`
- `[embed.go:AvailableSkills]`
- `[embed.go:Install]`

## Invariants
- None

## Side Effects
- [embed.go:87]: `if err := os.WriteFile(relPath, content, 0o644); err != nil {`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/agent/...`
