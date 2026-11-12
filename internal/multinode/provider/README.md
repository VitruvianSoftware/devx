# Provider

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/multinode/provider`.

## Interface
- `[lima.go:LimaProvider]`
- `[lima.go:NewLimaProvider]`
- `[lima.go:EnsureRuntime]`
- `[lima.go:InstallServer]`
- `[lima.go:InstallAgent]`
- `[lima.go:GetToken]`
- `[lima.go:IsInstalled]`
- `[lima.go:WaitForReady]`
- `[lima.go:NodeStatus]`
- `[lima.go:Kubeconfig]`
- `[lima.go:Drain]`
- `[lima.go:DeleteNode]`
- `[lima.go:Uninstall]`
- `[lima.go:Destroy]`
- `[lima.go:Reconcile]`
- `[native.go:NativeProvider]`
- `[native.go:NewNativeProvider]`
- `[native.go:EnsureRuntime]`
- `[native.go:InstallAgent]`
- `[native.go:InstallServer]`

## Invariants
- None

## Side Effects
- [native.go:147]: `// in installScript), so on a STOCK enforcing host systemd cannot exec the fr...`
- [native.go:148]: `// downloaded k3s binary (labeled user_tmp_t) → "Permission denied" (203/EXEC...`
- [native.go:276]: ``curl -sfL https://get.k3s.io | %s INSTALL_K3S_EXEC="server" sh -s - --server...`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/multinode/provider/...`
