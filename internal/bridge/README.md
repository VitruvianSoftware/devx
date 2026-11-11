# Bridge

## Overview
Component package providing implementation for `apps/cli/devx/internal/bridge`.

## Interface
- `[agent.go:AgentConfig]`
- `[agent.go:AgentInfo]`
- `[agent.go:DeployAgent]`
- `[agent.go:RemoveAgent]`
- `[env.go:GenerateEnvFile]`
- `[env.go:LoadEnvVars]`
- `[intercept.go:ServicePortSpec]`
- `[intercept.go:ServiceInfo]`
- `[intercept.go:ServiceState]`
- `[intercept.go:InspectService]`
- `[intercept.go:ValidateInterceptable]`
- `[intercept.go:CheckInterceptConflict]`
- `[intercept.go:PatchServiceSelector]`
- `[intercept.go:RestoreServiceSelector]`
- `[intercept.go:GetServiceSelector]`
- `[kube.go:ValidateKubectl]`
- `[kube.go:ResolveKubeconfig]`
- `[kube.go:ValidateContext]`
- `[kube.go:ValidateService]`
- `[kube.go:ListServices]`

## Invariants
- None

## Side Effects
- [agent.go:26]: `"os/exec"`
- [agent.go:93]: `cmd := exec.Command("kubectl", args...)`
- [agent.go:133]: `_ = exec.Command("kubectl", args...).Run()`
- [env.go:54]: `fmt.Sprintf("%s_URL=http://127.0.0.1:%d", prefix, e.LocalPort),`
- [env.go:62]: `return os.WriteFile(envPath, []byte(content), 0o644)`
- [intercept.go:26]: `"os/exec"`
- [intercept.go:34]: `Name       string `json:"name"`        // e.g., "http-api", "metrics" (may be...`
- [intercept.go:36]: `TargetPort string `json:"target_port"` // Container port number or name (e.g....`
- [kube.go:25]: `// Design: All cluster interactions use the kubectl subprocess pattern, consi...`
- [kube.go:37]: `"os/exec"`

## Verification
- Run: `bazel test //apps/cli/devx/internal/bridge/...`
