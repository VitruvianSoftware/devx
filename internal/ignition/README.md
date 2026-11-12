# Ignition

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/ignition`.

## Interface
- `[builder.go:Build]`
- `[builder.go:Validate]`
- `[deploy.go:SSHFunc]`
- `[deploy.go:Deploy]`
- `[deploy.go:DeployWithSSH]`

## Invariants
- None

## Side Effects
- [builder.go:27]: `"os/exec"`
- [builder.go:65]: `defer func() { _ = os.Remove(buFile.Name()) }() // clean up input after compi...`
- [builder.go:83]: `cmd := exec.Command("butane", "--pretty", "--strict", "--output", ignPath, bu...`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/ignition/...`
