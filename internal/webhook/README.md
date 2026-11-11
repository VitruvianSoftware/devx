# Webhook

## Overview
Component package providing implementation for `apps/cli/devx/internal/webhook`.

## Interface
- `[server.go:Request]`
- `[server.go:Server]`
- `[server.go:New]`
- `[server.go:Requests]`
- `[server.go:Start]`
- `[server.go:Shutdown]`
- `[tui.go:Model]`
- `[tui.go:NewModel]`
- `[tui.go:Init]`
- `[tui.go:Update]`
- `[tui.go:View]`
- `[tui.go:SendRequest]`

## Invariants
- None

## Side Effects
- [server.go:21]: `// Package webhook implements a local HTTP catch-all server for inspecting`
- [server.go:34]: `"net/http"`
- [server.go:56]: `srv      *http.Server`

## Verification
- Run: `bazel test //apps/cli/devx/internal/webhook/...`
