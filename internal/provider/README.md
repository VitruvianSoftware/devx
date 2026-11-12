# Provider

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/provider`.

## Interface
- `[colima.go:ColimaProvider]`
- `[colima.go:Name]`
- `[colima.go:Init]`
- `[colima.go:Start]`
- `[colima.go:StopAll]`
- `[colima.go:Sleep]`
- `[colima.go:Resize]`
- `[colima.go:Remove]`
- `[colima.go:SetDefault]`
- `[colima.go:SSH]`
- `[colima.go:IsRunning]`
- `[colima.go:Inspect]`
- `[detect.go:DetectedProvider]`
- `[detect.go:Detect]`
- `[detect.go:Resolve]`
- `[detect.go:MultipleProvidersError]`
- `[detect.go:Error]`
- `[docker.go:DockerProvider]`
- `[docker.go:Name]`
- `[docker.go:Init]`

## Invariants
- None

## Side Effects
- [colima.go:26]: `"os/exec"`
- [colima.go:40]: `cmd := exec.Command(`
- [colima.go:55]: `cmd := exec.Command("colima", "start", "--profile", name)`
- [detect.go:26]: `"os/exec"`
- [detect.go:60]: `path, err := exec.LookPath(binary)`
- [detect.go:122]: `if _, err := exec.LookPath(binary); err != nil {`
- [docker.go:27]: `"os/exec"`
- [docker.go:42]: `cmd := exec.Command("docker", "info")`
- [docker.go:52]: `cmd := exec.Command("docker", "info")`
- [lima.go:27]: `"os/exec"`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/provider/...`
