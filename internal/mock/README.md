# Mock

## Overview
Component package providing implementation for `apps/cli/devx/internal/mock`.

## Interface
- `[server.go:MockServer]`
- `[server.go:Up]`
- `[server.go:MockInfo]`
- `[server.go:List]`
- `[server.go:Restart]`
- `[server.go:Remove]`
- `[server.go:WaitForReady]`
- `[server.go:EnvKey]`

## Invariants
- None

## Side Effects
- [server.go:26]: `// to HTTP requests according to the remote OpenAPI spec it is initialized with.`
- [server.go:30]: `//	MOCK_<NAME>_URL=http://localhost:<port>`
- [server.go:38]: `"net/http"`

## Verification
- Run: `bazel test //apps/cli/devx/internal/mock/...`
