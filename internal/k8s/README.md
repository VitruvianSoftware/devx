# K8S

## Overview
Component package providing implementation for `apps/cli/devx/internal/k8s`.

## Interface
- `[k3s.go:Cluster]`
- `[k3s.go:Spawn]`
- `[k3s.go:List]`
- `[k3s.go:Remove]`

## Invariants
- None

## Side Effects
- [k3s.go:30]: `"os/exec"`
- [k3s.go:51]: `// Spawn boots a new k3s container and provisions its isolated kubeconfig.`
- [k3s.go:52]: `func Spawn(runtime, name string) (*Cluster, error) {`

## Verification
- Run: `bazel test //apps/cli/devx/internal/k8s/...`
