# Cloud

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/cloud`.

## Interface
- `[emulator.go:Emulator]`
- `[emulator.go:Registry]`
- `[emulator.go:SupportedServices]`
- `[emulator.go:EnvVarValues]`

## Invariants
- None

## Side Effects
- [emulator.go:22]: `// container configurations for one-command local provisioning via devx cloud...`
- [emulator.go:55]: `"STORAGE_EMULATOR_HOST": "http://%s",`
- [emulator.go:101]: `"AWS_ENDPOINT_URL_S3":   "http://%s",`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/cloud/...`
