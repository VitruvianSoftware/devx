# Trafficproxy

## Overview
Component package providing implementation for `apps/cli/devx/internal/trafficproxy`.

## Interface
- `[proxy.go:Profile]`
- `[proxy.go:Profiles]`
- `[proxy.go:Start]`

## Invariants
- None

## Side Effects
- [proxy.go:68]: `l, err := net.Listen("tcp", "127.0.0.1:0")`
- [proxy.go:96]: `port := l.Addr().(*net.TCPAddr).Port`
- [proxy.go:100]: `func handleConnection(ctx context.Context, src net.Conn, targetPort string, p...`

## Verification
- Run: `bazel test //apps/cli/devx/internal/trafficproxy/...`
