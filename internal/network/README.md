# Network

## Overview
Component package providing implementation for `apps/cli/devx/internal/network`.

## Interface
- `[ports.go:CheckPortAvailable]`
- `[ports.go:GetFreePort]`
- `[ports.go:ResolvePort]`

## Invariants
- None

## Side Effects
- [ports.go:30]: `ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))`
- [ports.go:40]: `ln, err := net.Listen("tcp", "127.0.0.1:0")`
- [ports.go:46]: `addr := ln.Addr().(*net.TCPAddr)`

## Verification
- Run: `bazel test //apps/cli/devx/internal/network/...`
