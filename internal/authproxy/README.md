# Authproxy

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/authproxy`.

## Interface
- `[proxy.go:Start]`

## Invariants
- None

## Side Effects
- [proxy.go:27]: `"net/http"`
- [proxy.go:28]: `"net/http/httputil"`
- [proxy.go:33]: `// Start spins up a local HTTP reverse proxy that adds Basic Authentication`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/authproxy/...`
