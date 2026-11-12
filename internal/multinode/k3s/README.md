# K3S

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/multinode/k3s`.

## Interface
- `[advanced.go:GetVersion]`
- `[advanced.go:ReinstallServer]`
- `[advanced.go:ReinstallJoinServer]`
- `[advanced.go:ReinstallAgent]`
- `[advanced.go:UncordonNode]`
- `[advanced.go:IsNodeReady]`
- `[advanced.go:CreateSnapshot]`
- `[advanced.go:DownloadSnapshot]`
- `[advanced.go:UploadSnapshot]`
- `[advanced.go:RestoreSnapshot]`
- `[advanced.go:StopService]`
- `[advanced.go:StartService]`
- `[cilium.go:ReadNodePodCIDR]`
- `[cilium.go:AdvertiseNodeRoutes]`
- `[cilium.go:LBRangeCIDR]`
- `[k3s.go:Manager]`
- `[k3s.go:NewManager]`
- `[k3s.go:NewManagerWithVM]`
- `[k3s.go:NewManagerWithExec]`
- `[k3s.go:IsInstalled]`

## Invariants
- None

## Side Effects
- [advanced.go:59]: ``curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q INSTALL_K3S_EXEC="serv...`
- [advanced.go:71]: ``curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q K3S_TOKEN=%q INSTALL_K...`
- [advanced.go:83]: ``curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION=%q K3S_TOKEN=%q K3S_URL=%...`
- [cilium.go:37]: `// exec), and the node must already have joined so its podCIDR is assigned.`
- [cilium.go:58]: `// (the Manager's exec is bound to that node). podCIDR is the node's pod CIDR;`
- [k3s.go:45]: `// Manager handles K3s operations on a node, running privileged commands via ...`
- [k3s.go:51]: `exec   execFunc`
- [k3s.go:67]: `exec: func(ctx context.Context, cmd string) (string, error) {`
- [recipe.go:99]: ``curl -sfL https://get.k3s.io | %sINSTALL_K3S_EXEC="server" sh -s - --cluster...`
- [recipe.go:109]: ``curl -sfL https://get.k3s.io | %sINSTALL_K3S_SKIP_START=true K3S_TOKEN=%q IN...`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/multinode/k3s/...`
