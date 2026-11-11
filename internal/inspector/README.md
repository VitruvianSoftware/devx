# Inspector

## Overview
Component package providing implementation for `apps/cli/devx/internal/inspector`.

## Interface
- `[capture.go:CapturedExchange]`
- `[proxy.go:Proxy]`
- `[proxy.go:NewProxy]`
- `[proxy.go:ListenPort]`
- `[proxy.go:Start]`
- `[proxy.go:Close]`
- `[proxy.go:ServeHTTP]`
- `[proxy.go:Exchanges]`
- `[proxy.go:Clear]`
- `[proxy.go:Replay]`
- `[proxy.go:WriteHeader]`
- `[proxy.go:Write]`
- `[proxy.go:Flush]`
- `[tui.go:CaptureMsg]`
- `[tui.go:TUIModel]`
- `[tui.go:NewTUIModel]`
- `[tui.go:Init]`
- `[tui.go:Update]`
- `[tui.go:View]`

## Invariants
- None

## Side Effects
- [capture.go:21]: `// Package inspector provides an HTTP reverse proxy with request/response`
- [capture.go:26]: `"net/http"`
- [capture.go:30]: `// CapturedExchange holds a recorded HTTP request/response pair.`
- [proxy.go:28]: `"net/http"`
- [proxy.go:29]: `"net/http/httputil"`
- [proxy.go:40]: `// Proxy is a reverse proxy that captures HTTP request/response exchanges.`
- [tui.go:25]: `"net/http"`
- [tui.go:472]: `func renderHeaders(h http.Header) string {`

## Verification
- Run: `bazel test //apps/cli/devx/internal/inspector/...`
