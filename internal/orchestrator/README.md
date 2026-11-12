# Orchestrator

## Overview
Component package providing implementation for `apps/cli/vitruvian-devx/internal/orchestrator`.

## Interface
- `[bridge_node.go:BridgeNodeState]`
- `[bridge_node.go:Cleanup]`
- `[bridge_node.go:GenerateBridgeEnvVars]`
- `[bridge_node.go:WriteBridgeEnvFile]`
- `[bridge_node.go:BridgeServicesJSON]`
- `[dag.go:Runtime]`
- `[dag.go:BridgeMode]`
- `[dag.go:BridgeNodeConfig]`
- `[dag.go:KubeNodeConfig]`
- `[dag.go:CloudRunNodeConfig]`
- `[dag.go:ContainerNodeConfig]`
- `[dag.go:HealthcheckConfig]`
- `[dag.go:DependsOnEntry]`
- `[dag.go:ServiceConfig]`
- `[dag.go:NodeType]`
- `[dag.go:Node]`
- `[dag.go:DAG]`
- `[dag.go:NewDAG]`
- `[dag.go:AddNode]`
- `[dag.go:Validate]`

## Invariants
- None

## Side Effects
- [bridge_node.go:54]: `// 4. Stop port-forward subprocess`
- [bridge_node.go:147]: `// We spawn it in a goroutine so the DAG tier's wg.Wait() can proceed.`
- [bridge_node.go:366]: `// Generate BRIDGE_<SERVICE_NAME>_URL=http://localhost:<port>`
- [cloudrun_node.go:27]: `"os/exec"`
- [cloudrun_node.go:156]: `cmd := exec.CommandContext(logCtx, "gcloud", cloudRunLogsTailArgs(c.Project, ...`
- [cloudrun_node.go:163]: `out, err := exec.CommandContext(ctx, "gcloud", args...).CombinedOutput()`
- [container_node.go:84]: `// cleanup that removes the container on shutdown. The generic HTTP/TCP`
- [container_node.go:115]: `_, _ = rt.Exec("rm", "-f", name)`
- [container_node.go:170]: `_, _ = rt.Exec("rm", "-f", containerNodeName(n.Name))`
- [dag.go:29]: `"net/http"`

## Verification
- Run: `bazel test //apps/cli/vitruvian-devx/internal/orchestrator/...`
