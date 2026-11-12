# Tailscale

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/tailscale`.

## Interface
- `[agent.go:SSHFunc]`
- `[agent.go:WaitForDaemon]`
- `[agent.go:WaitForDaemonWithSSH]`
- `[agent.go:Up]`
- `[agent.go:UpWithSSH]`
- `[agent.go:Status]`
- `[agent.go:StatusWithSSH]`
- `[agent.go:ExtractAuthURL]`

## Invariants
- None

## Side Effects
- [agent.go:51]: `return fmt.Errorf("tailscaled did not start within %v — check: devx exec %s l...`
- [agent.go:63]: `fmt.Sprintf("sudo %s exec tailscaled timeout 5 tailscale up --accept-routes -...`
- [agent.go:81]: `fmt.Sprintf("sudo %s exec tailscaled tailscale status --self 2>/dev/null | he...`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/tailscale/...`
