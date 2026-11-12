# Config

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/multinode/config`.

## Interface
- `[config.go:Config]`
- `[config.go:ClusterConfig]`
- `[config.go:USBConfig]`
- `[config.go:WiFiConfig]`
- `[config.go:TailscaleConfig]`
- `[config.go:MetalLBConfig]`
- `[config.go:LoadBalancerConfig]`
- `[config.go:LBIPRange]`
- `[config.go:LBL2Hostnames]`
- `[config.go:DockerConfig]`
- `[config.go:CiliumConfig]`
- `[config.go:MountConfig]`
- `[config.go:NodeConfig]`
- `[config.go:GetKind]`
- `[config.go:VMConfig]`
- `[config.go:GetVMName]`
- `[config.go:Load]`
- `[config.go:ServerNodes]`
- `[config.go:AgentNodes]`
- `[config.go:InitNode]`

## Invariants
- None

## Side Effects
- None

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/multinode/config/...`
