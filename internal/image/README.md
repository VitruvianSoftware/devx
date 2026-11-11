# Image

## Overview
Component package providing implementation for `apps/cli/devx/internal/image`.

## Interface
- `[build.go:Build]`
- `[buildload.go:BuildAndLoad]`
- `[image.go:Spec]`
- `[image.go:TagOrDefault]`
- `[image.go:ContextOrDefault]`
- `[image.go:DockerfileOrDefault]`
- `[push.go:Push]`
- `[registry.go:LocalRegistry]`
- `[registry.go:Ref]`
- `[registry.go:PushRef]`
- `[registry.go:EnsureRegistry]`

## Invariants
- None

## Side Effects
- [push.go:32]: `// insecure-registry flag so pushes to the in-cluster HTTP registry succeed.`
- [registry.go:26]: `"os/exec"`
- [registry.go:150]: `http:`
- [registry.go:156]: `regionendpoint: http://minio-amd64.minio.svc.cluster.local:9000`

## Verification
- Run: `bazel test //apps/cli/devx/internal/image/...`
