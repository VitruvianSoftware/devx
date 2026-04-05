# Idea 46.3: Full Hybrid Topology (`runtime: bridge` in `devx up`)

> **Status: ✅ APPROVED — Ready for Execution**
> All design decisions have been reviewed and approved by the product owner.

## Background

With Ideas 46.1 (outbound bridge) and 46.2 (inbound intercept) shipped, the bridge features exist as **standalone CLI commands** — `devx bridge connect` and `devx bridge intercept` must be run separately from `devx up`. Idea 46.3 closes the loop by making bridge operations first-class participants in the `devx up` orchestration DAG.

**Today's workflow** (3 terminals):
```bash
# Terminal 1: start local stack
devx up

# Terminal 2: connect to remote staging services
devx bridge connect

# Terminal 3: intercept traffic  
devx bridge intercept payments-api --steal
```

**After 46.3** (1 terminal):
```bash
# Everything orchestrated together
devx up
```

### Design Principle Alignment

- **One CLI, Everything** — Bridge operations should not require separate invocations when declared in `devx.yaml`
- **Client-Driven Architecture** — All bridge operations remain ephemeral and client-controlled
- **Declarative Idempotency** — `devx.yaml` fully describes the hybrid topology

---

## User Review Required

> [!IMPORTANT]
> **`runtime: bridge` — New Service Runtime Value**
>
> Services with `runtime: bridge` are treated as remote K8s services managed by the bridge subsystem, not local processes. This is a new enum value alongside `host`, `container`, `kubernetes`, and `cloud`. Services with this runtime skip local process spawning and instead trigger bridge connect or intercept operations.

> [!WARNING]
> **Intercept services in `devx up` run with `--steal` by default**
>
> When an intercept is defined in `devx.yaml` and orchestrated by `devx up`, the `--steal` mode is implied. There is no interactive confirmation since the intent is declared in the YAML. This means a `devx up` can silently redirect staging traffic. The `--dry-run` flag on `devx up` will show all bridge operations that would be performed.

> [!IMPORTANT]
> **Shutdown Ordering: Bridge teardown happens AFTER local services stop**
>
> When the user presses Ctrl+C, the teardown order is:
> 1. Stop local services (graceful SIGTERM → SIGKILL)
> 2. Restore intercepted Service selectors
> 3. Remove agent pods
> 4. Kill port-forward tunnels
> 5. Clean session files
>
> This prevents a brief window where the intercepted service could receive traffic meant for the local app after it's already dead. The agent's self-healing is the safety net if the CLI is killed before completing step 2.

---

## Proposed Changes

### Architecture Overview

```
┌─ devx up ──────────────────────────────────────────────────────────────────┐
│                                                                           │
│  1. resolveConfig("devx.yaml", profile)                                   │
│     ├── Databases      → devx db spawn (existing)                         │
│     ├── Services                                                          │
│     │   ├── runtime: host       → startHostProcess() (existing)           │
│     │   ├── runtime: container  → docker/podman run (existing)            │
│     │   ├── runtime: bridge     → NEW: startBridgeService()               │
│     │   │   ├── bridge_mode: connect  → bridge.PortForward (outbound)     │
│     │   │   └── bridge_mode: intercept → bridge.DeployAgent + Yamux       │
│     │   └── runtime: kubernetes → kubectlApply (existing)                 │
│     └── Tunnels        → cloudflared (existing)                           │
│                                                                           │
│  2. DAG orchestration — bridge services participate in dependency graph    │
│     ├── Tier 0: postgres (database)                                       │
│     ├── Tier 1: payments-api (bridge/connect), redis-staging (bridge)     │
│     ├── Tier 2: local-api (host, depends_on: [postgres, payments-api])    │
│     └── Tier 3: web (host, depends_on: [local-api])                       │
│                                                                           │
│  3. Unified shutdown — reverse teardown with bridge cleanup               │
└───────────────────────────────────────────────────────────────────────────┘
```

### Schema Extension

---

#### [MODIFY] [devxconfig.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/devxconfig.go)

**What changes:** Add `runtime: bridge` support to `DevxConfigService` by introducing bridge-specific fields that work alongside the existing service schema.

```yaml
# devx.yaml — 46.3 schema additions
services:
  # Outbound bridge: remote service available locally 
  - name: payments-api
    runtime: bridge
    bridge_target:
      service: payments-api        # K8s service name
      namespace: staging           # optional (default: bridge.namespace)
      port: 8080                   # remote port
      local_port: 8080             # optional: local port binding
    healthcheck:
      tcp: "localhost:8080"        # validates bridge is forwarding
      timeout: "15s"

  # Inbound intercept: steal traffic from remote to local
  - name: user-service-intercept
    runtime: bridge
    bridge_intercept:
      service: user-service        # K8s service to intercept
      namespace: staging
      port: 3000
      local_port: 3000
      mode: steal                  # required: "steal" (mirror: future)
    depends_on:
      - name: local-user-svc       # local service must be ready first
        condition: service_healthy

  # Local service that depends on the bridged remote service
  - name: local-api
    runtime: host
    command: ["go", "run", "./cmd/api"]
    port: 9090
    depends_on:
      - name: payments-api          # bridge service — waits for port-forward to be healthy
        condition: service_healthy
      - name: postgres
        condition: service_healthy
```

Go schema additions:

```go
// DevxConfigBridgeTarget — already exists (Idea 46.1), reused for inline service targets

// DevxConfigServiceBridgeTarget defines an inline bridge connect target on a service.
type DevxConfigServiceBridgeTarget struct {
    Service   string `yaml:"service"`    // K8s service name
    Namespace string `yaml:"namespace"`  // Override namespace
    Port      int    `yaml:"port"`       // Remote service port
    LocalPort int    `yaml:"local_port"` // Local port to bind (0 = auto)
}

// DevxConfigServiceBridgeIntercept defines an inline bridge intercept on a service.
type DevxConfigServiceBridgeIntercept struct {
    Service   string `yaml:"service"`    // K8s service to intercept
    Namespace string `yaml:"namespace"`  // Override namespace
    Port      int    `yaml:"port"`       // Remote service port
    LocalPort int    `yaml:"local_port"` // Local port to route traffic to
    Mode      string `yaml:"mode"`       // "steal" or "mirror" (required)
}

// DevxConfigService (extended)
type DevxConfigService struct {
    // ... existing fields unchanged ...
    Runtime         string                            `yaml:"runtime"` // "host", "container", "kubernetes", "cloud", "bridge"
    BridgeTarget    *DevxConfigServiceBridgeTarget    `yaml:"bridge_target,omitempty"`    // NEW: inline outbound bridge
    BridgeIntercept *DevxConfigServiceBridgeIntercept `yaml:"bridge_intercept,omitempty"` // NEW: inline intercept
}
```

**Validation rules** (in `resolveConfig` or a new `validateBridgeServices()` helper):
1. If `runtime: bridge`, exactly one of `bridge_target` or `bridge_intercept` must be set
2. If `bridge_intercept`, `mode` must be `"steal"` (or error with "mirror not yet implemented")
3. If `bridge_intercept`, a top-level `bridge:` section must exist (for kubeconfig/context)
4. If `bridge_target`, either a top-level `bridge:` section must exist OR `bridge_target.service` must be specified
5. Port must be > 0
6. Service name uniqueness is enforced by the existing merge logic (no changes needed)

---

#### [MODIFY] [dag.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/orchestrator/dag.go)

**What changes:** Add `RuntimeBridge` as a new `Runtime` constant so the DAG recognizes bridge services. Add two new node fields: `BridgeMode` (connect vs intercept) and `BridgeConfig` (opaque config passed through to the bridge subsystem). Extend `Execute()` to handle `RuntimeBridge` nodes by calling a new `startBridgeNode()` function.

```go
const (
    RuntimeHost       Runtime = "host"
    RuntimeContainer  Runtime = "container"
    RuntimeKubernetes Runtime = "kubernetes"
    RuntimeCloud      Runtime = "cloud"
    RuntimeBridge     Runtime = "bridge"     // NEW: Idea 46.3
)

// BridgeMode distinguishes between outbound and inbound bridge operations.
type BridgeMode string

const (
    BridgeModeConnect   BridgeMode = "connect"   // Outbound: kubectl port-forward
    BridgeModeIntercept BridgeMode = "intercept"  // Inbound: agent + yamux tunnel
)

// BridgeNodeConfig holds bridge-specific configuration for a DAG node.
type BridgeNodeConfig struct {
    Kubeconfig       string
    Context          string
    Namespace        string
    TargetService    string
    RemotePort       int
    LocalPort        int
    AgentImage       string
    Mode             BridgeMode
    InterceptMode    string // "steal" or "mirror"
}

// Node (extended)
type Node struct {
    // ... existing fields ...
    BridgeMode   BridgeMode       // NEW: for RuntimeBridge nodes
    BridgeConfig *BridgeNodeConfig // NEW: bridge-specific parameters
}
```

**Dispatch guard refactor:** The existing `Execute()` goroutine (dag.go line 223) guards on `n.Type == NodeService && len(n.Command) > 0`. Bridge nodes have `len(n.Command) == 0` (they use `BridgeConfig` instead), so they would be **silently skipped** under the current guard. This must be refactored into a runtime switch:

```go
// Inside the Execute() goroutine — REPLACES the existing if-guard at dag.go:223
if n.Type == NodeService {
    switch n.Runtime {
    case RuntimeBridge:
        if err := startBridgeNode(ctx, n); err != nil {
            errCh <- fmt.Errorf("failed to start bridge service %q: %w", n.Name, err)
            return
        }
    default:
        if len(n.Command) > 0 {
            // ... existing port resolution + startHostProcess ...
        }
    }
}
```

`startBridgeNode` lives in a new file to keep the DAG clean (see below).

---

#### [NEW] `internal/orchestrator/bridge_node.go`

**Purpose:** Implements `startBridgeNode()` — the DAG executor's handler for `RuntimeBridge` services. This is the integration glue between the DAG orchestrator and the `internal/bridge/` package.

```go
package orchestrator

import (
    "context"
    "fmt"

    "github.com/VitruvianSoftware/devx/internal/bridge"
)

// BridgeNodeState holds the runtime state of a running bridge node, used for teardown.
type BridgeNodeState struct {
    PortForward *bridge.PortForward  // non-nil for connect mode
    Tunnel      *bridge.Tunnel       // non-nil for intercept mode
    AgentInfo   *bridge.AgentInfo    // non-nil for intercept mode
    SvcState    *bridge.ServiceState // non-nil for intercept mode
    SessionID   string               // non-nil for intercept mode
}

// startBridgeNode dispatches to connect or intercept based on bridge mode.
func startBridgeNode(ctx context.Context, n *Node) error {
    if n.BridgeConfig == nil {
        return fmt.Errorf("bridge node %q has no BridgeConfig", n.Name)
    }
    
    cfg := n.BridgeConfig

    kcPath, err := bridge.ResolveKubeconfig(cfg.Kubeconfig)
    if err != nil {
        return err
    }
    if err := bridge.ValidateContext(kcPath, cfg.Context); err != nil {
        return err
    }

    switch cfg.Mode {
    case BridgeModeConnect:
        return startBridgeConnect(ctx, n, kcPath, cfg)
    case BridgeModeIntercept:
        return startBridgeIntercept(ctx, n, kcPath, cfg)
    default:
        return fmt.Errorf("unknown bridge mode %q for node %q", cfg.Mode, n.Name)
    }
}

func startBridgeConnect(ctx context.Context, n *Node, kcPath string, cfg *BridgeNodeConfig) error {
    // Validate service exists
    if err := bridge.ValidateService(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService); err != nil {
        return err
    }

    pf := bridge.NewPortForward(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService, cfg.RemotePort, cfg.LocalPort)

    if warning, err := pf.ResolveLocalPort(); err != nil {
        return err
    } else if warning != "" {
        fmt.Println(warning)
    }

    // Update node's resolved port for healthcheck
    n.Port = pf.LocalPort

    // Store for cleanup
    n.bridgeState = &BridgeNodeState{PortForward: pf}

    fmt.Printf("  🔗 Bridging %s → %s/%s:%d → localhost:%d\n",
        n.Name, cfg.Namespace, cfg.TargetService, cfg.RemotePort, pf.LocalPort)

    // CRITICAL: pf.Start() blocks until ctx is cancelled (it runs kubectl port-forward
    // with a retry loop — see portforward.go:98). We MUST spawn it in a goroutine
    // so the DAG tier's wg.Wait() can proceed. Readiness is tracked via pf.State()
    // and polled by waitForHealthy() using the bridge-native health path.
    go func() {
        _ = pf.Start(ctx)
    }()

    return nil
}

func startBridgeIntercept(ctx context.Context, n *Node, kcPath string, cfg *BridgeNodeConfig) error {
    // Intercept setup has two phases:
    //
    // Phase A (SYNCHRONOUS — finite operations, ~10-30s total):
    //   1. Inspect + validate service
    //   2. Deploy agent pod (waits for Running)
    //   3. Patch service selector
    //   4. Establish Yamux tunnel connection
    //
    // Phase B (GOROUTINE — blocks forever):
    //   5. Tunnel maintenance loop (keepalives, stream proxying)
    //
    // When startBridgeIntercept returns nil, the intercept IS fully operational.
    // No separate healthcheck phase is needed — the finite setup steps ARE the
    // health verification. This differs from connect mode where the port-forward
    // is spawned asynchronously and readiness is polled via pf.State().
    //
    // A naive TCP healthcheck on localhost:<local_port> would give FALSE POSITIVES
    // for intercept nodes because the local developer app owns that port, not the
    // bridge. The port being open says nothing about whether the tunnel is up.

    // 1. Inspect + validate
    info, err := bridge.InspectService(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService)
    if err != nil {
        return err
    }
    if err := bridge.ValidateInterceptable(info); err != nil {
        return err
    }

    // 2. Deploy agent
    sessionID := uuid.New().String()[:8]
    agentCfg := bridge.AgentConfig{
        Kubeconfig:       kcPath,
        Context:          cfg.Context,
        Namespace:        cfg.Namespace,
        TargetService:    cfg.TargetService,
        InterceptPort:    cfg.RemotePort,
        ServicePorts:     info.Ports,
        OriginalSelector: info.Selector,
        AgentImage:       cfg.AgentImage,
        SessionID:        sessionID,
    }
    agentInfo, err := bridge.DeployAgent(agentCfg)
    if err != nil {
        return err
    }

    // 3. Patch selector
    agentSelector := map[string]string{
        "devx-bridge":         "agent",
        "devx-bridge-session": sessionID,
    }
    svcState, err := bridge.PatchServiceSelector(kcPath, cfg.Context, cfg.Namespace, cfg.TargetService, agentSelector, sessionID)
    if err != nil {
        _ = bridge.RemoveAgent(kcPath, cfg.Context, cfg.Namespace, sessionID)
        return err
    }

    // 4. Start tunnel (synchronous — establishes connection)
    tunnel := bridge.NewTunnel(bridge.TunnelConfig{
        Kubeconfig:  kcPath,
        Context:     cfg.Context,
        Namespace:   cfg.Namespace,
        AgentPod:    agentInfo.PodName,
        ControlPort: agentInfo.ControlPort,
        LocalPort:   cfg.LocalPort,
    })
    if err := tunnel.Start(ctx); err != nil {
        _ = bridge.RestoreServiceSelector(kcPath, cfg.Context, svcState)
        _ = bridge.RemoveAgent(kcPath, cfg.Context, cfg.Namespace, sessionID)
        return err
    }

    // Store state for DAG cleanup
    n.bridgeState = &BridgeNodeState{
        Tunnel:    tunnel,
        AgentInfo: agentInfo,
        SvcState:  svcState,
        SessionID: sessionID,
    }

    fmt.Printf("  🔀 Intercepting %s/%s:%d → localhost:%d (steal)\n",
        cfg.Namespace, cfg.TargetService, cfg.RemotePort, cfg.LocalPort)

    // 5. Persist session with DAG origin
    interceptEntry := bridge.InterceptEntry{
        Service:          cfg.TargetService,
        Namespace:        cfg.Namespace,
        TargetPort:       cfg.RemotePort,
        LocalPort:        cfg.LocalPort,
        Mode:             "steal",
        AgentPod:         agentInfo.PodName,
        SessionID:        sessionID,
        OriginalSelector: info.Selector,
        StartedAt:        time.Now(),
        Origin:           "dag",
    }
    session, _ := bridge.LoadSession()
    if session == nil {
        session = &bridge.Session{
            Kubeconfig: kcPath,
            Context:    cfg.Context,
            StartedAt:  time.Now(),
        }
    }
    session.Intercepts = append(session.Intercepts, interceptEntry)
    _ = bridge.SaveSession(session)

    // Returning nil = intercept is fully operational.
    // The tunnel maintenance loop runs inside tunnel.Start's internal goroutines.
    return nil
}
```

> [!NOTE]
> **Code Reuse Strategy**
>
> `startBridgeIntercept` calls the same `bridge.*` functions as `cmd/bridge_intercept.go` — no logic is duplicated. The DAG handler is a thin orchestration wrapper that omits CLI output formatting.

> [!WARNING]
> **Blocking Behavior — Connect vs Intercept**
>
> The two bridge modes have fundamentally different blocking characteristics:
> - **Connect**: `pf.Start()` blocks forever (runs `kubectl port-forward` with retry loop). Must be goroutined. Readiness is polled via `pf.State() == StateHealthy`.
> - **Intercept**: Setup steps (deploy agent, patch selector, open tunnel) are **finite** (10-30s). Only the tunnel maintenance runs forever, but it runs inside `tunnel.Start`'s internal goroutines. `startBridgeIntercept` returns nil when the intercept is fully operational — **no separate healthcheck needed**.
>
> A naive TCP healthcheck on `localhost:<local_port>` for intercept nodes would give **false positives** because the local developer app owns that port, not the bridge.

Extend the `Node` struct with a `bridgeState` field for cleanup:

```go
type Node struct {
    // ... existing fields ...
    bridgeState *BridgeNodeState // NEW: runtime state for bridge cleanup
}
```

Extend the DAG cleanup function to handle bridge teardown:

```go
cleanupFn := func() {
    for i := len(startedNodes) - 1; i >= 0; i-- {
        n := startedNodes[i]
        
        // Bridge cleanup: restore selectors, remove agents, stop port-forwards
        if n.bridgeState != nil {
            if n.bridgeState.SvcState != nil {
                _ = bridge.RestoreServiceSelector(...)
            }
            if n.bridgeState.AgentInfo != nil {
                _ = bridge.RemoveAgent(...)
            }
            if n.bridgeState.Tunnel != nil {
                n.bridgeState.Tunnel.Stop()
            }
            if n.bridgeState.PortForward != nil {
                n.bridgeState.PortForward.Stop()
            }
        }
        
        // Existing process cleanup
        if n.cancel != nil {
            n.cancel()
        }
        if n.process != nil && n.process.Process != nil {
            _ = n.process.Process.Kill()
        }
    }
}
```

---

#### [MODIFY] [up.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/up.go)

**What changes:** When building the service DAG, detect `runtime: bridge` services and construct their `BridgeNodeConfig` from the inline `bridge_target` or `bridge_intercept` fields merged with the top-level `bridge:` config. Also generate `BRIDGE_*_URL` env vars into `~/.devx/bridge.env` after bridge services are healthy.

Specific modifications:

1. **Service registration loop** (around line 217-259): Add a `case "bridge":` that sets `rt = orchestrator.RuntimeBridge` and populates `BridgeMode` + `BridgeConfig` on the node.

2. **Post-DAG execution**: After `dag.Execute()` succeeds, scan started nodes for bridge services and call `bridge.GenerateEnvFile()` with resolved bridge entries so `devx shell` picks them up.

3. **Empty check guard** (line 51): Update the empty check to also consider bridge services:
   ```go
   if len(cfgYaml.Tunnels) == 0 && len(cfgYaml.Databases) == 0 && len(cfgYaml.Services) == 0 {
   ```
   This already works since bridge services are regular `services:` entries — no change needed.

4. **Validation**: Add a new `validateBridgeServices()` call after config resolution to detect misconfigured bridge services early (before the DAG is built).

```go
// In the service registration loop:
case "bridge":
    rt = orchestrator.RuntimeBridge
    
    bridgeCfg := cfgYaml.Bridge
    if bridgeCfg == nil {
        return fmt.Errorf("service %q uses runtime: bridge but no top-level 'bridge:' section is defined in devx.yaml", svc.Name)
    }
    
    var bridgeMode orchestrator.BridgeMode
    var bcfg *orchestrator.BridgeNodeConfig
    
    if svc.BridgeTarget != nil {
        bridgeMode = orchestrator.BridgeModeConnect
        ns := svc.BridgeTarget.Namespace
        if ns == "" { ns = bridgeCfg.Namespace }
        if ns == "" { ns = "default" }
        bcfg = &orchestrator.BridgeNodeConfig{
            Kubeconfig:    bridgeCfg.Kubeconfig,
            Context:       bridgeCfg.Context,
            Namespace:     ns,
            TargetService: svc.BridgeTarget.Service,
            RemotePort:    svc.BridgeTarget.Port,
            LocalPort:     svc.BridgeTarget.LocalPort,
            Mode:          orchestrator.BridgeModeConnect,
        }
    } else if svc.BridgeIntercept != nil {
        bridgeMode = orchestrator.BridgeModeIntercept
        // validate mode
        if svc.BridgeIntercept.Mode != "steal" {
            return fmt.Errorf("service %q: bridge intercept mode must be 'steal' (mirror not yet implemented)", svc.Name)
        }
        ns := svc.BridgeIntercept.Namespace
        if ns == "" { ns = bridgeCfg.Namespace }
        if ns == "" { ns = "default" }
        bcfg = &orchestrator.BridgeNodeConfig{
            Kubeconfig:    bridgeCfg.Kubeconfig,
            Context:       bridgeCfg.Context,
            Namespace:     ns,
            TargetService: svc.BridgeIntercept.Service,
            RemotePort:    svc.BridgeIntercept.Port,
            LocalPort:     svc.BridgeIntercept.LocalPort,
            AgentImage:    bridgeCfg.AgentImage,
            Mode:          orchestrator.BridgeModeIntercept,
            InterceptMode: svc.BridgeIntercept.Mode,
        }
    } else {
        return fmt.Errorf("service %q has runtime: bridge but neither bridge_target nor bridge_intercept is defined", svc.Name)
    }
    
    // ... node := &orchestrator.Node{ ..., BridgeMode: bridgeMode, BridgeConfig: bcfg }
```

---

#### [MODIFY] [bridge_connect.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_connect.go)

**What changes:** No functional changes. The standalone `devx bridge connect` command continues to work independently. However, we extract `resolveBridgeConfig()` into a shared helper so `up.go` can also resolve bridge configuration without duplicating logic.

Actually, on re-examination, the top-level `bridge:` config is already read by `resolveConfig()` in `devxconfig.go` and available on the `DevxConfig.Bridge` field. No extraction needed — `up.go` already has access via `cfgYaml.Bridge`.

---

#### [MODIFY] [bridge_intercept.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_intercept.go)

**What changes:** Extract the core intercept lifecycle (steps 6-19 in `runBridgeIntercept`) into an exported function in `internal/bridge/` so the DAG orchestrator can reuse it without importing `cmd/`. This avoids circular imports and respects the established `cmd/ → internal/` dependency direction.

New function in `internal/bridge/intercept.go`:

```go
// RunIntercept executes the full intercept lifecycle and returns state for cleanup.
// This is the reusable core — called by both `cmd/bridge_intercept.go` (standalone)
// and `internal/orchestrator/bridge_node.go` (DAG integration).
func RunIntercept(ctx context.Context, cfg InterceptConfig) (*InterceptState, error) {
    // ... mirrors runBridgeIntercept steps 6-16, returns state for cleanup
}

// InterceptConfig contains all parameters for an intercept operation.
type InterceptConfig struct {
    Kubeconfig  string
    Context     string
    Namespace   string
    Service     string
    Port        int
    LocalPort   int
    AgentImage  string
    Mode        string // "steal"
}

// InterceptState holds runtime state for cleanup.
type InterceptState struct {
    Tunnel      *Tunnel
    AgentInfo   *AgentInfo
    SvcState    *ServiceState 
    SessionID   string
    Entry       InterceptEntry
}

// CleanupIntercept restores the service and removes the agent.
func CleanupIntercept(state *InterceptState) error {
    // 1. Stop tunnel
    // 2. Restore selector
    // 3. Remove agent
    // 4. Remove from session file
}
```

Then `cmd/bridge_intercept.go` is refactored to call `bridge.RunIntercept()` + CLI output formatting, reducing its size by ~50%.

---

#### [MODIFY] [session.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/bridge/session.go)

**What changes:** Add an `Origin` field to `SessionEntry` and `InterceptEntry` to distinguish between standalone bridge sessions and DAG-managed sessions. This prevents `devx bridge disconnect` from accidentally tearing down DAG-managed bridges (which are owned by the `devx up` process).

```go
type SessionEntry struct {
    // ... existing fields ...
    Origin string `json:"origin,omitempty"` // "standalone" or "dag" — prevents cross-teardown
}

type InterceptEntry struct {
    // ... existing fields ...
    Origin string `json:"origin,omitempty"` // "standalone" or "dag"
}
```

`devx bridge disconnect` will skip entries with `Origin: "dag"` and print:
```
⚠️  Skipping DAG-managed bridge 'payments-api' — managed by 'devx up'. Stop devx up to teardown.
```

---

#### [MODIFY] [bridge_disconnect.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_disconnect.go)

**What changes:** Filter out `Origin: "dag"` entries from the disconnect operation with a descriptive warning.

---

### Documentation Updates

---

#### [MODIFY] [docs/guide/bridge.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/bridge.md)

Major additions:
- New "Hybrid Topology (`devx up` Integration)" section explaining `runtime: bridge`
- Complete YAML examples for connect and intercept services in the DAG
- Dependency graph examples showing bridge → local service chains
- Shutdown ordering documentation
- Interaction with profiles (e.g., `--profile staging` enables bridge services)

#### [MODIFY] [docs/guide/orchestration.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/orchestration.md)

Update the service runtime table to include `bridge` with a brief explanation and link to bridge docs.

#### [MODIFY] [docs/guide/architecture.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/architecture.md)

Update "Idea 46.3 (Future)" to "Idea 46.3 (Shipped)" with architectural notes.

#### [MODIFY] [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example)

Add full bridge service examples (both connect and intercept) with inline comments.

#### [MODIFY] [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md)

Add Idea 46.3 feature entry.

#### [MODIFY] [IDEAS.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/IDEAS.md)

Mark 46.3 status as "Implemented."

#### [MODIFY] [.agent/skills/devx/SKILL.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/.agent/skills/devx/SKILL.md) + all 5 template locations

Add `runtime: bridge` awareness to the `devx up` section.

#### [MODIFY] [README.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/README.md)

Update the feature list to reflect unified hybrid topology.

---

## Gap Analysis

| Area | Current State | Gap for 46.3 | Resolution |
|------|--------------|--------------|------------|
| **Service runtime** | `host`, `container`, `kubernetes`, `cloud` | No `bridge` runtime | Add `RuntimeBridge` to DAG orchestrator |
| **DAG dispatch guard** | `Execute()` guards on `len(n.Command) > 0` | Bridge nodes have no `Command` — silently skipped! | Refactor guard into a `switch n.Runtime` so bridge nodes dispatch to `startBridgeNode()` |
| **Connect blocking** | `startHostProcess` calls `cmd.Start()` (non-blocking) | `pf.Start()` blocks forever (`cmd.Wait()` in retry loop) | Spawn `pf.Start()` in goroutine; poll readiness via `pf.State()` |
| **Intercept blocking** | N/A (new) | Agent deploy + selector patch are finite (10-30s); tunnel maintenance blocks forever | Run finite setup synchronously, tunnel maintenance runs in internal goroutines; return nil = healthy |
| **Intercept healthcheck** | N/A (new) | TCP check on `localhost:<local_port>` gives false positives — local app owns the port | No separate healthcheck needed for intercept; `startBridgeIntercept` returning nil IS the readiness signal |
| **Connect healthcheck** | DAG uses raw TCP/HTTP polling | `pf.State()` tracks health natively via internal goroutine (portforward.go:166-180) | Add bridge-native health path to `waitForHealthy()` that polls `pf.State()` |
| **Bridge config in services** | Bridge config is a separate top-level section | Services need inline bridge config | Add `bridge_target` and `bridge_intercept` fields to `DevxConfigService` |
| **Intercept reusability** | Intercept lifecycle is hardcoded in `cmd/bridge_intercept.go` | DAG needs to call intercept without importing `cmd/` | Inline the calls to `bridge.*` functions directly in `startBridgeIntercept()` |
| **DAG cleanup** | Only kills processes | Bridge services need selector restore + agent removal | Extend cleanup with `BridgeNodeState` teardown |
| **Session ownership** | Sessions don't track who created them | `devx bridge disconnect` could kill DAG-managed bridges | Add `Origin` field to session entries |
| **Env file generation** | `bridge connect` generates `bridge.env` | `devx up` needs to generate it after bridge nodes are healthy | Add post-DAG env file generation in `up.go` |
| **Validation** | No validation for `runtime: bridge` | Must validate that `bridge_target`/`bridge_intercept` is set | Add `validateBridgeServices()` in config resolution |
| **Profile merge** | Profiles merge services by name | Bridge-specific fields need merging | Extend `mergeProfile()` to handle `BridgeTarget`/`BridgeIntercept` fields |

---

## Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| `runtime: bridge` without top-level `bridge:` section | Fail-fast with: "service 'X' uses runtime: bridge but no top-level 'bridge:' section is defined" |
| `runtime: bridge` with neither `bridge_target` nor `bridge_intercept` | Fail-fast with: "service 'X' has runtime: bridge but neither bridge_target nor bridge_intercept is defined" |
| `runtime: bridge` with both `bridge_target` AND `bridge_intercept` | Fail-fast with: "service 'X' cannot have both bridge_target and bridge_intercept — use separate services" |
| Bridge service port collision | Auto-shift via existing `network.ResolvePort()` (same as all other services) |
| Cluster unreachable during `devx up` | Existing exit code 61 (`CodeBridgeContextUnreachable`) — fails the DAG tier, triggers full cleanup |
| RBAC insufficient for intercept in `devx up` | Exit code 68 — DAG cleanup tears down any already-started intercepts |
| `Ctrl+C` during bridge node startup | DAG cleanup runs in reverse: restore selectors → remove agents → kill port-forwards |
| `devx bridge disconnect` while `devx up` is running | Skips DAG-managed entries with warning, only tears down standalone bridges |
| `devx up --dry-run` with bridge services | Shows bridge operations that would be performed (outbound targets, intercept targets) without modifying the cluster |
| Bridge service healthcheck timeout | Existing healthcheck timeout handling — DAG rolls back the entire tier |
| Profile overrides a service from `host` to `bridge` runtime | Merge works: profile's `runtime: bridge` + `bridge_target` replaces the base service's fields |
| Multiple intercepts in one `devx up` | Each gets its own agent pod + session ID + Yamux tunnel — all managed and cleaned up independently |
| `devx up` killed with SIGKILL (untrappable) | Agent self-healing restores selectors within 30s (existing 46.2 safety net) |
| Agent image pull fails (air-gapped cluster) | Exit code 65 → DAG tier fails → cleanup → suggests `bridge.agent_image` in `devx.yaml` |

---

## Resolved Decisions (From User Review)

> [!TIP]
> **Q1: Top-level `bridge:` section vs `services:` with `runtime: bridge` — ✅ Kept Separate**
>
> The `services:` array is for DAG-orchestrated bridge operations. The top-level `bridge.targets` and `bridge.intercepts` arrays remain standalone-only (used by `devx bridge connect`/`devx bridge intercept`). This is explicit and avoids confusion about which section controls what.

> [!TIP]
> **Q2: kubectl Validation — ✅ Lazy (On First Bridge Node)**
>
> `devx up` validates `kubectl` availability lazily — only when the DAG encounters its first `RuntimeBridge` node during execution. This keeps `devx up` fast for non-bridge topologies. `devx doctor` already validates kubectl availability upfront for developers who want pre-flight checks.

> [!TIP]
> **Intercept in `devx up` implies `--steal` — ✅ Accepted**
>
> When an intercept is defined in `devx.yaml` and orchestrated by `devx up`, the `--steal` mode is implied from the YAML declaration. No interactive confirmation is needed since the intent is declarative. `devx up --dry-run` previews all bridge operations without modifying the cluster.

---

## Verification Plan

### Automated Tests

```bash
# Run the full CI pipeline
devx action ci
```

#### Unit Tests to Write

- `internal/orchestrator/bridge_node_test.go` — Test `startBridgeNode()` config resolution for both connect and intercept modes, error handling for missing config
- `internal/orchestrator/dag_test.go` — Extend existing DAG tests with `RuntimeBridge` nodes in the dependency graph (validate ordering, validate cleanup is called in reverse)
- `cmd/devxconfig_test.go` — Extend existing tests with:
  - `runtime: bridge` + `bridge_target` (valid)
  - `runtime: bridge` + `bridge_intercept` (valid)
  - `runtime: bridge` without bridge config (error)
  - `runtime: bridge` with both target and intercept (error)
  - Profile merge overriding `runtime: host` → `runtime: bridge` (valid)
- `internal/bridge/intercept_test.go` — Test the extracted `RunIntercept()` function
- `cmd/up_test.go` — Test that `validateBridgeServices()` catches config errors

#### Integration Testing (Manual — requires a cluster)

```bash
# 1. Spawn a local k3s cluster
devx k8s spawn test-hybrid --json -y

# 2. Deploy echo + redis into the cluster
kubectl apply -f test-fixtures/echo-service.yaml

# 3. Create a devx.yaml with hybrid topology
cat > /tmp/test-hybrid/devx.yaml <<EOF
name: hybrid-test
bridge:
  kubeconfig: ~/.kube/devx-test-hybrid.yaml
  namespace: default
databases:
  - engine: postgres
    port: 5432
services:
  - name: remote-echo
    runtime: bridge
    bridge_target:
      service: echo
      port: 5678
    healthcheck:
      tcp: "localhost:5678"
      timeout: "15s"
  - name: local-api
    runtime: host
    command: ["python3", "-m", "http.server", "9090"]
    port: 9090
    depends_on:
      - name: postgres
        condition: service_healthy
      - name: remote-echo
        condition: service_healthy
    healthcheck:
      http: "http://localhost:9090"
      timeout: "30s"
EOF

# 4. Run devx up
cd /tmp/test-hybrid && devx up

# 5. In another terminal: verify remote-echo is bridged
curl localhost:5678  # Should return response from cluster echo service

# 6. Verify local-api started AFTER bridge was healthy
curl localhost:9090  # Should return directory listing from python server

# 7. Ctrl+C — verify clean shutdown (bridge.env cleaned, no session leftovers)

# 8. Cleanup
devx k8s rm test-hybrid -y
```

### Edge Case Scenarios to Verify

| Scenario | Test |
|----------|------|
| `devx up --dry-run` with bridge services | Must show planned bridge operations without modifying the cluster |
| `devx up --profile staging` enabling bridge services | Profile with `runtime: bridge` must correctly resolve and start |
| `devx bridge disconnect` during `devx up` | Must skip DAG-managed entries with warning |
| RBAC failure mid-DAG | Must cleanup already-started bridge nodes gracefully |
| Multiple bridge services in different tiers | Must start in correct order and cleanup in reverse |
| `devx up` with both bridge and non-bridge services | Non-bridge services must not be affected by bridge failures |

### Build Validation

```bash
# Dogfood the local CLI — never run raw go commands directly.
devx action ci
```

All verification MUST use `devx action ci` per the established dogfooding standard (SKILL.md §9). Raw `go vet`, `go test`, and `go build` commands are forbidden.
