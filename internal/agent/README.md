# Agent

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/agent`.

## Interface
- `[embed.go:EmbedFS]`
- `[embed.go:AgentBasePaths]`
- `[embed.go:AvailableSkills]`
- `[embed.go:CheckedInSkillDirs]`
- `[embed.go:SkillTemplatePath]`
- `[embed.go:SkillContent]`
- `[embed.go:Install]`

## Invariants
- One canonical template file exists per skill (`templates/skills/<id>/SKILL.md`), shared across all agent targets.

## Side Effects
- [embed.go:103]: `if err := os.WriteFile(relPath, content, 0o644); err != nil {`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/agent/...`
