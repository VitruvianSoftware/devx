# Lima

## Overview
Component package providing implementation for `apps/cli/devx/internal/multinode/lima`.

## Interface
- `[keeper.go:NodeKeeperLabel]`
- `[keeper.go:InstallNodeKeeper]`
- `[lima.go:VMStatus]`
- `[lima.go:NodePackages]`
- `[lima.go:Manager]`
- `[lima.go:NewManager]`
- `[lima.go:Status]`
- `[lima.go:GenerateConfig]`
- `[lima.go:Provision]`
- `[lima.go:GetBridgedIP]`
- `[lima.go:Destroy]`
- `[mounts.go:DefaultMountType]`
- `[mounts.go:ResolveMounts]`
- `[mounts.go:Spec]`
- `[mounts.go:ParseSpec]`
- `[mounts.go:MountsEqual]`
- `[mounts.go:MountsRewriteScript]`

## Invariants
- None

## Side Effects
- [keeper.go:64]: `<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DT...`
- [keeper.go:105]: `// launchd exec's ProgramArguments directly (no shell, no ~ expansion), so the`
- [lima.go:53]: `const NodePackages = "curl open-iscsi nfs-common socat"`
- [lima.go:156]: `// `curl get.docker.com | sh`). docker-ce + containerd.io are installed at`
- [lima.go:163]: `curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/...`

## Verification
- Run: `bazel test //apps/cli/devx/internal/multinode/lima/...`
