# Podman

## Overview
Component package providing implementation for `apps/cli/devx/internal/podman`.

## Interface
- `[machine.go:MachineInfo]`
- `[machine.go:StopAll]`
- `[machine.go:Remove]`
- `[machine.go:Init]`
- `[machine.go:Start]`
- `[machine.go:SetDefault]`
- `[machine.go:SSH]`
- `[machine.go:IsRunning]`
- `[machine.go:Inspect]`

## Invariants
- None

## Side Effects
- [machine.go:27]: `"os/exec"`
- [machine.go:39]: `cmd := exec.Command("podman", "machine", "stop")`
- [machine.go:46]: `cmd := exec.Command("podman", "machine", "rm", "-f", name)`

## Verification
- Run: `bazel test //apps/cli/devx/internal/podman/...`
