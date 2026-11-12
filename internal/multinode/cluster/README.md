# Cluster

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/multinode/cluster`.

## Interface
- `[advanced.go:Upgrade]`
- `[advanced.go:Backup]`
- `[advanced.go:Restore]`
- `[apply.go:Apply]`
- `[cluster.go:InitOptions]`
- `[cluster.go:Init]`
- `[cluster.go:Join]`
- `[cluster.go:Reconcile]`
- `[cluster.go:Remove]`
- `[cluster.go:Destroy]`
- `[cluster.go:Status]`

## Invariants
- [cluster.go:464]: `// Cluster-level defaults: re-assert CoreDNS HA + single default StorageClass`

## Side Effects
- None

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/multinode/cluster/...`
