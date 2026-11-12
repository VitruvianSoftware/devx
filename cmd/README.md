# Cmd

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/cmd`.

## Interface
- `[debug_export.go:GetRootCmd]`
- `[devxconfig.go:DevxConfigInclude]`
- `[devxconfig.go:DevxConfigDatabaseSeed]`
- `[devxconfig.go:DevxConfigDatabasePull]`
- `[devxconfig.go:DevxConfigDatabase]`
- `[devxconfig.go:DevxConfigTest]`
- `[devxconfig.go:DevxConfigTestUI]`
- `[devxconfig.go:DevxConfigMock]`
- `[devxconfig.go:DevxConfigDependsOn]`
- `[devxconfig.go:DevxConfigHealthcheck]`
- `[devxconfig.go:DevxConfigSync]`
- `[devxconfig.go:DevxConfigTunnel]`
- `[devxconfig.go:DevxConfigServiceBridgeTarget]`
- `[devxconfig.go:DevxConfigServiceBridgeIntercept]`
- `[devxconfig.go:DevxConfigKubernetesImage]`
- `[devxconfig.go:DevxConfigKubernetesSync]`
- `[devxconfig.go:DevxConfigServiceKubernetes]`
- `[devxconfig.go:DevxConfigServiceCloudRun]`
- `[devxconfig.go:DevxConfigContainerBuild]`
- `[devxconfig.go:DevxConfigServiceContainer]`

## Invariants
- [cluster_mgmt_reconcile.go:41]: `set — including socat, which kubectl port-forward and devx bridge require to`
- [mcp_uninstall.go:43]: `Antigravity) require manual removal — devx will print instructions.`
- [nuke.go:187]: `fmt.Println("  Some items may require sudo — run: sudo devx nuke")`
- [state_attach.go:149]: `Message:  "This bundle contains CRIU checkpoints which require Podman, but yo...`
- [state_share.go:70]: `fmt.Println("⚠️  Warning: CRIU checkpoints require Podman. Auto-enabling --db...`
- [vm_autoresume.go:36]: `// that require the local hypervisor/provider to be un-suspended.`

## Side Effects
- [action.go:28]: `"os/exec"`
- [action.go:205]: `command := exec.Command(cmdArgs[0], cmdArgs[1:]...)`
- [action.go:227]: `if exitErr, ok := err.(*exec.ExitError); ok {`
- [agent.go:26]: `"os/exec"`
- [agent.go:135]: `ollamaPath, err := exec.LookPath("ollama")`
- [agent.go:145]: `if _, err := exec.LookPath(ollamaName); err == nil {`
- [agent_review.go:26]: `"os/exec"`
- [agent_review.go:313]: `diff, err := exec.Command("git", "diff", "--cached", "--stat").CombinedOutput()`
- [agent_review.go:315]: `diff, err = exec.Command("git", "diff", "--stat").CombinedOutput()`
- [agent_ship.go:28]: `"os/exec"`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/cmd/...`
