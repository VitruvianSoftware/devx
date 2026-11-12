# Mcpserver

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/mcpserver`.

## Interface
- `[server.go:ServerName]`
- `[server.go:NewServer]`
- `[server.go:AllToolNames]`

## Invariants
- None

## Side Effects
- [exec.go:24]: `// agent host. Subprocess isolation keeps the MCP server's own stdout (which`
- [exec.go:34]: `"os/exec"`
- [exec.go:51]: `cmd := exec.CommandContext(ctx, devxBinary, args...)`
- [tools.go:100]: `out := []string{"db", "spawn", engine, "-y", "--json"}`
- [tools.go:120]: `out := []string{"cloud", "spawn", svc}`
- [tools.go:298]: `// annotations and a shared subprocess handler.`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/mcpserver/...`
