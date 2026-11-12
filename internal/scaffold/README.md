# Scaffold

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/scaffold`.

## Interface
- `[engine.go:EmbedFS]`
- `[engine.go:Vars]`
- `[engine.go:Template]`
- `[engine.go:Registry]`
- `[engine.go:Find]`
- `[engine.go:Result]`
- `[engine.go:Scaffold]`
- `[engine.go:PostScaffold]`
- `[engine.go:GitAuthor]`
- Subdirectory: [templates](./templates/README.md)

## Invariants
- None

## Side Effects
- [engine.go:28]: `"os/exec"`
- [engine.go:67]: `Description: "Production-ready Go HTTP API — Chi router, PostgreSQL, migratio...`
- [engine.go:82]: `Description: "TypeScript Express API — Prisma ORM, PostgreSQL, Jest, Docker, ...`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/scaffold/...`
