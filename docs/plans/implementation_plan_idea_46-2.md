# Idea 46.2: Inbound Traffic Interception (`devx bridge intercept`)

> **Status: ✅ APPROVED — Ready for Execution**
> All design decisions have been reviewed and approved by the product owner.

## Background

Idea 46.1 (shipped) provides **outbound** connectivity — local code can call remote K8s services via port-forward tunnels. Idea 46.2 completes the other half: routing **inbound** cluster traffic to the developer's local machine, enabling live debugging against real staging requests.

This is the core value proposition that Telepresence and mirrord deliver. We are implementing it within the `devx bridge` command family using the **Client-Driven Architecture** principle: the agent is ephemeral, auto-cleaning, and entirely controlled from the developer's local `devx` CLI.

### Competitive Landscape

| Tool | Agent Type | Lifecycle | Capabilities Required | Client Dependency |
|------|-----------|-----------|----------------------|-------------------|
| **Telepresence v2** | Sidecar (injected via MutatingWebhook) | Permanent Traffic Manager + ephemeral Agent | `CAP_NET_ADMIN`, iptables, cluster-wide webhook | Daemon on workstation |
| **mirrord** | K8s Job OR Ephemeral Container | Fully ephemeral, no pre-installed infra | `CAP_NET_ADMIN`, `CAP_NET_RAW`, `CAP_SYS_PTRACE`, `CAP_SYS_ADMIN` | LD_PRELOAD library |
| **devx bridge (Idea 46.2)** | K8s Job | Fully ephemeral, auto-deadline cleanup | `CAP_NET_ADMIN`, `CAP_NET_RAW` | kubectl subprocess |

### Key Architectural Choice: K8s Job Agent (Not Ephemeral Container)

> [!IMPORTANT]
> **Design Decision: Kubernetes Job over Ephemeral Container**
>
> While K8s ephemeral containers share the target pod's network namespace "for free," they have critical limitations:
> 1. **Cannot define ports** — we need to expose a tunnel endpoint
> 2. **Cannot set resource requests** — risk eviction under pressure
> 3. **Cannot be removed once injected** — they persist until the pod restarts
> 4. **Require `pods/ephemeralcontainers` RBAC** — many clusters restrict this
>
> Instead, we deploy a standalone **K8s Job** with `activeDeadlineSeconds` (auto-cleanup) that runs on the *same node* as the target pod via `nodeSelector`. The agent captures traffic using iptables rules applied to the target pod's service via a temporary `EndpointSlice` override, then tunnels it back to the developer's machine via a reverse `kubectl port-forward` to the agent pod.

---

## Resolved Decisions (From User Review)

> [!TIP]
> **Agent Container Image — ✅ Approved (Configurable)**
>
> Default: `ghcr.io/VitruvianSoftware/devx-bridge-agent:<version>`, published via a dedicated GitHub Actions workflow.
> The image is **configurable** via `--agent-image` CLI flag and `bridge.agent_image` in `devx.yaml`, allowing users in air-gapped or private environments to point to their own pre-loaded registry. The default image constant is set at build time in the CLI binary.

> [!TIP]
> **Explicit `--steal` Flag Required — ✅ Approved**
>
> Running `devx bridge intercept <service>` without a mode flag will error with:
> `"mode required: specify --steal (redirects all traffic) or --mirror (not yet available)"`
> This forces developers to explicitly acknowledge the destructive nature of traffic redirection.

> [!TIP]
> **Decoupled Agent Module — ✅ Approved**
>
> The agent binary lives in a separate `internal/bridge/agent/` directory with its own `go.mod` and a dedicated GitHub Actions workflow for building and publishing the container image. It ships independently from the CLI and only changes when the agent protocol changes.

> [!TIP]
> **Separate `intercepts:` Key in `devx.yaml` — ✅ Approved**
>
> Intercept targets are defined under `bridge.intercepts:` (separate from outbound `bridge.targets:`). This keeps the schema explicit about direction and mode.

> [!WARNING]
> **RBAC Requirements — Cluster Permissions**
>
> The intercept feature requires the developer's kubeconfig to have these permissions in the target namespace:
> - `create/delete` on `jobs` (deploy the agent)
> - `get/list` on `pods`, `services`, `endpoints`, `endpointslices` (discovery + validation)
> - `update` on `services` (to temporarily reroute the service selector during `--steal` mode)
> - `create/delete` on `configmaps` (session metadata for multi-developer coordination)
>
> This is a significantly higher privilege bar than 46.1 (which only needed `get` on services + port-forward). We will document the minimum RBAC ClusterRole in our docs and provide a `devx bridge rbac` helper that generates the required YAML.

> [!NOTE]
> **Service Mesh Compatibility (Istio / Linkerd) — Unsupported in 46.2**
>
> Service mesh environments are declared out of scope for 46.2. If a mesh sidecar is detected (Istio/Linkerd annotations on target pod), `devx bridge intercept` will print a warning but proceed. Full mesh awareness is deferred to a future enhancement.

---

## Proposed Changes

### Architecture Overview

```
┌────────────────────────────────────────────────────────────────────────────┐
│  Developer's Machine                                                       │
│                                                                            │
│  ┌──────────────┐   localhost    ┌───────────────────────────────────────┐ │
│  │ Local Service │◄─────────────│ devx bridge intercept (orchestrator)  │ │
│  │ (port 8080)   │              │                                       │ │
│  └──────────────┘              │ 1. kubectl port-forward to agent:4200 │ │
│                                 │ 2. Dials agent control port           │ │
│                                 │ 3. Yamux session over single TCP conn │ │
│                                 │ 4. Each inbound stream → localhost    │ │
│                                 └──────┬────────────────────────────────┘ │
│                                         │ kubectl port-forward             │
└─────────────────────────────────────────┼──────────────────────────────────┘
                                          │
┌─────────────────────────────────────────┼──────────────────────────────────┐
│  Kubernetes Cluster (staging)           │                                  │
│                                         ▼                                  │
│  ┌──────────────────────────────────────────────────────────────────┐      │
│  │ devx-bridge-agent Job (auto-generated Pod spec)                  │      │
│  │  • Listens on control port :4200 (Yamux server)                  │      │
│  │  • Mirrors target Service's containerPorts (names + numbers)     │      │
│  │  • On inbound request: opens Yamux stream → CLI → localhost      │      │
│  │  • Self-healing: restores Service selector on tunnel drop/SIGTERM│      │
│  │  • activeDeadlineSeconds: 14400 (4h auto-cleanup)                │      │
│  │  • Runs with dedicated ServiceAccount (narrow RBAC)              │      │
│  └──────────────────────────────────────────────────────────────────┘      │
│                                                                            │
│  ┌─────────────────────┐     ┌──────────────────────┐                     │
│  │ Original Pod         │     │ Service              │                     │
│  │ (payments-api)       │     │ (selector patched    │                     │
│  │ [traffic diverted]   │     │  during intercept)   │                     │
│  └─────────────────────┘     └──────────────────────┘                     │
└────────────────────────────────────────────────────────────────────────────┘
```

### Data Plane: Yamux Multiplexed Tunnel

Kubernetes `kubectl port-forward` is **unidirectional** (local → pod). To route cluster traffic back to the developer's machine, we use a **client-initiated, server-multiplexed** tunnel:

1. **`devx` CLI** establishes a standard `kubectl port-forward <agent-pod> <local-port>:4200`
2. **`devx` CLI** dials `localhost:<local-port>`, creating a TCP connection to the agent's control port
3. **Agent** accepts this connection and creates a `hashicorp/yamux` **server** session over it
4. When a cluster client sends a request to the intercepted Service port, the **agent** opens a new Yamux stream
5. **`devx` CLI** receives the stream on the Yamux **client** session, dials `localhost:<local-port>`, and proxies bytes bidirectionally
6. Response bytes flow back: local app → CLI → Yamux stream → agent → cluster client

> [!NOTE]
> **Why Yamux?**
>
> - Battle-tested multiplexer used by HashiCorp Consul, Nomad, and Vault
> - Pure Go, zero CGO — compatible with our `distroless` agent image
> - Supports concurrent streams over a single TCP connection (critical for HTTP/2 and parallel requests)
> - 8-byte frame overhead per stream — negligible for debugging workloads
> - Alternative considered: `golang.org/x/net/http2` — rejected because we need raw TCP proxying, not just HTTP

### Phase Breakdown (within 46.2)

| Sub-Phase | Scope | Risk |
|-----------|-------|------|
| **46.2a** | Agent image + deploy/cleanup lifecycle + `--steal` mode only | Low |
| **46.2b** | `--mirror` mode (duplicate traffic, originals still served) | Medium |
| **46.2c** | Multi-developer coordination (ConfigMap lock, `--header` routing) | Medium |

This plan covers **46.2a** — the core `--steal` (full redirect) mode. Mirror mode and multi-developer coordination are deferred to follow-up sub-phases.

---

### Agent Image

#### [NEW] `internal/bridge/agent/` (separate Go module)

A minimal Go binary (~5MB) that:

1. **Control port** — Listens on `:4200` for the Yamux control connection from the `devx` CLI
2. **Service ports** — Dynamically opens listeners on ports matching the target Service's `containerPorts` (including named ports), configured via CLI args at Job deploy time
3. **Yamux server** — When a cluster client connects to a service port, the agent opens a new Yamux stream over the control connection and proxies bytes bidirectionally
4. **Health endpoint** — Exposes `/healthz` on `:4201` (separate from control port) for readiness probing
5. **Self-healing** — On tunnel disconnect or `SIGTERM`, the agent **restores the original Service selector** before exiting, using its dedicated `ServiceAccount`. The original selector is passed via the `DEVX_ORIGINAL_SELECTOR` environment variable (JSON-encoded `map[string]string`)
6. **Heartbeat** — Sends Yamux keepalives every 10s. If no response within 30s, considers the client dead and triggers self-healing

The agent does NOT modify iptables. Traffic redirection is achieved by patching the Kubernetes Service's selector to point at the agent pod (the "selector swap" approach).

> [!NOTE]
> **Why Selector Swap over iptables?**
>
> - iptables requires `CAP_NET_ADMIN` + `CAP_SYS_ADMIN` + shared network namespace
> - Selector swap only requires `update` on `services` — a standard RBAC permission
> - Selector swap works regardless of network CNI (Calico, Cilium, etc.)
> - Telepresence v1 used this approach before switching to sidecar injection in v2
> - Trade-off: during `--steal`, the original pod receives zero traffic (acceptable for debugging)

> [!IMPORTANT]
> **Self-Healing Agent Pattern**
>
> If the developer's laptop dies or `devx` is `SIGKILL`'d, the Service selector would remain patched indefinitely — breaking staging for the whole team. To prevent this:
> - The agent runs with a **dedicated `ServiceAccount`** that has a narrow RBAC role: `update` on the specific target Service only
> - The `devx` CLI passes the original selector via `DEVX_ORIGINAL_SELECTOR` env var and the target service/namespace via `DEVX_TARGET_SERVICE` / `DEVX_TARGET_NAMESPACE`
> - The agent monitors the Yamux connection health. On disconnect, `SIGTERM`, or `activeDeadlineSeconds` timeout, it **restores the original selector before exiting**
> - This makes crash recovery fully automatic — no manual `devx bridge disconnect --force` required

#### Agent Image Build

```dockerfile
FROM gcr.io/distroless/static-debian12:nonroot
COPY devx-bridge-agent /
ENTRYPOINT ["/devx-bridge-agent"]
```

Published via a dedicated GitHub Actions workflow to `ghcr.io/VitruvianSoftware/devx-bridge-agent:<version>`. Version is pinned in the `devx` CLI binary at build time via an `AgentImageDefault` constant.

Users can override the default with `--agent-image` or `bridge.agent_image` in `devx.yaml`:

```yaml
bridge:
  agent_image: my-registry.example.com/devx-bridge-agent:v0.1.0  # optional override
  intercepts:
    - service: payments-api
      port: 8080
      mode: steal
```

---

### CLI Commands

#### [NEW] [bridge_intercept.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_intercept.go)

New subcommand: `devx bridge intercept <service>`

```
devx bridge intercept payments-api --steal
devx bridge intercept payments-api --steal --port 8080 --local-port 8080
devx bridge intercept payments-api --steal --agent-image my-registry/agent:v1
devx bridge intercept payments-api --mirror  # errors: "not yet implemented"
devx bridge intercept payments-api --steal --dry-run
devx bridge intercept payments-api --steal --json
```

**Lifecycle:**
1. Resolve kubeconfig/context/namespace (reuse `resolveBridgeConfig()`)
2. Validate target service exists (reuse `bridge.ValidateService()`)
3. **Validate service type**: reject `ExternalName` services and services with empty selectors
4. **Validate port protocol**: reject services with `protocol: UDP` ports (TCP-only in 46.2)
5. **Inspect target service**: retrieve full Service spec including selector, all ports (names, numbers, protocols), and `targetPort` mappings
6. Discover the target service's current selector labels
7. Deploy the agent Job to the cluster:
   - Image: resolved from `--agent-image` flag → `bridge.agent_image` config → `AgentImageDefault` constant
   - **Dynamic Pod spec**: `containerPorts` mirror the target Service's port definitions (including named ports like `http-api`)
   - Agent args: `--control-port=4200 --ports=8080:http-api,9090:metrics` (dynamically generated)
   - Env vars: `DEVX_ORIGINAL_SELECTOR`, `DEVX_TARGET_SERVICE`, `DEVX_TARGET_NAMESPACE`
   - **ServiceAccount**: `devx-bridge-agent-<session-id>` with a Role scoped to `update` on the target Service only
   - Labels: `devx-bridge=agent`, `devx-bridge-target=<service>`, `devx-bridge-session=<uuid>`
   - `activeDeadlineSeconds: 14400` (4 hours — auto-cleanup safety net)
   - `ttlSecondsAfterFinished: 60` (cleanup completed/failed Jobs quickly)
8. Wait for agent pod to be `Running` and `/healthz` on `:4201` to return 200
9. Patch the target Service's selector to point at the agent pod's labels
10. **Establish Yamux tunnel**: `kubectl port-forward <agent-pod> <local-rand>:4200` → dial → Yamux client session
11. **Start local proxy**: for each Yamux stream received, dial `localhost:<local-port>` and proxy bytes
12. Persist intercept session to `~/.devx/bridge.json` (extend existing schema)
13. Block until Ctrl+C
14. On shutdown: restore original Service selector → delete agent Job + ServiceAccount + Role + RoleBinding → clean session

**CLI flags:**
- `--port` / `-p` — Remote port to intercept (defaults to first port on the Service)
- `--local-port` — Local port where traffic is routed (default: same as --port)
- `--steal` — Full traffic redirect (**required** — no implicit default)
- `--mirror` — Duplicate traffic only, future 46.2b (errors with "not yet implemented")
- `--agent-image` — Override the default agent container image (for air-gapped/private registries)
- `--kubeconfig`, `--context`, `-n` — Reuse from bridge connect
- `--json`, `-y`, `--dry-run` — Global flag compliance

#### [MODIFY] [bridge.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge.go)

Register `bridgeInterceptCmd` and update the help text to include `intercept`.

#### [MODIFY] [bridge_disconnect.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_disconnect.go)

Extend `disconnect` to also clean up intercept sessions:
- Delete agent Jobs labeled `devx-bridge=agent`
- Restore patched Service selectors from session metadata
- Remove intercept entries from `bridge.json`

#### [MODIFY] [bridge_status.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_status.go)

Extend status display to show intercept sessions with mode indicator (`steal`/`mirror`).

#### [NEW] [bridge_rbac.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_rbac.go)

`devx bridge rbac` — prints the minimum RBAC ClusterRole/RoleBinding YAML required for bridge intercept to function. Supports `--namespace` to scope to a specific namespace.

---

### Internal Package Extensions

#### [NEW] [internal/bridge/agent.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/bridge/agent.go)

Manages the agent Job lifecycle:

```go
// ServicePortSpec captures a single port from the target Service for dynamic agent generation.
type ServicePortSpec struct {
    Name       string `json:"name"`        // e.g., "http-api", "metrics" (may be empty)
    Port       int    `json:"port"`        // Service port number
    TargetPort int    `json:"target_port"` // Container port number (resolved from named ports)
    Protocol   string `json:"protocol"`    // Must be "TCP" for 46.2
}

// AgentConfig defines the parameters for deploying a bridge agent.
type AgentConfig struct {
    Kubeconfig       string
    Context          string
    Namespace        string
    TargetService    string
    InterceptPort    int               // The specific port being intercepted
    ServicePorts     []ServicePortSpec // ALL ports on the target Service (for dynamic Pod spec)
    OriginalSelector map[string]string // For self-healing: passed to agent via env var
    AgentImage       string            // e.g., "ghcr.io/VitruvianSoftware/devx-bridge-agent:v0.1.0"
    SessionID        string            // UUID for labeling
    Deadline         int               // activeDeadlineSeconds (default: 14400)
}

// DeployAgent creates the agent Job, ServiceAccount, Role, and RoleBinding
// in the cluster and waits for the agent to be ready.
func DeployAgent(cfg AgentConfig) (*AgentInfo, error)

// RemoveAgent deletes the agent Job, ServiceAccount, Role, and RoleBinding.
func RemoveAgent(kubeconfig, context, namespace, sessionID string) error

// AgentInfo contains runtime information about a deployed agent.
type AgentInfo struct {
    PodName      string
    PodIP        string
    ControlPort  int    // Yamux control port (4200)
    HealthPort   int    // Health check port (4201)
    SessionID    string
}
```

`DeployAgent` dynamically generates the Job's Pod spec to mirror the target Service's ports:
- Each `ServicePortSpec` becomes a `containerPort` on the agent container (preserving `name` for named-port resolution)
- The agent args include `--ports=<port>:<name>,<port>:<name>` so the binary knows which ports to listen on
- A dedicated `ServiceAccount`, `Role` (scoped to `update` on the target Service), and `RoleBinding` are created and torn down with the Job

All kubectl interactions use `exec.Command("kubectl", ...)` — consistent with the codebase pattern.

#### [NEW] [internal/bridge/tunnel.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/bridge/tunnel.go)

Manages the Yamux tunnel lifecycle on the CLI side:

```go
// TunnelConfig defines parameters for establishing the Yamux tunnel.
type TunnelConfig struct {
    Kubeconfig   string
    Context      string
    Namespace    string
    AgentPod     string
    ControlPort  int    // Agent's Yamux control port (4200)
    LocalPort    int    // Developer's local app port
}

// Tunnel manages the kubectl port-forward subprocess and Yamux client session.
type Tunnel struct {
    // unexported fields: yamux.Session, port-forward cmd, etc.
}

// Start establishes the kubectl port-forward and Yamux client session.
// Returns after the first Yamux handshake succeeds.
func (t *Tunnel) Start(ctx context.Context) error

// Stop gracefully closes the Yamux session and kills the port-forward subprocess.
func (t *Tunnel) Stop()

// Healthy returns true if the Yamux session is still alive.
func (t *Tunnel) Healthy() bool
```

Internally, `Start()` launches `kubectl port-forward` as a subprocess, dials the local forwarded port, and creates a `yamux.Client` session. A background goroutine accepts new Yamux streams (each representing an inbound cluster request) and proxies them to `localhost:<LocalPort>`.

#### [NEW] [internal/bridge/intercept.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/bridge/intercept.go)

Manages the Service inspection, validation, and selector swap:

```go
// ServiceInfo captures the full spec of a target Service for intercept planning.
type ServiceInfo struct {
    Name             string            `json:"name"`
    Namespace        string            `json:"namespace"`
    Type             string            `json:"type"`              // ClusterIP, NodePort, etc.
    Selector         map[string]string `json:"selector"`
    Ports            []ServicePortSpec `json:"ports"`             // All ports with names/protocols
    HasMeshSidecar   bool              `json:"has_mesh_sidecar"` // Detected via annotations
}

// ServiceState captures the original state of a Service before patching (for restore).
type ServiceState struct {
    Name             string            `json:"name"`
    Namespace        string            `json:"namespace"`
    OriginalSelector map[string]string `json:"original_selector"`
}

// InspectService retrieves the full Service spec and validates it for intercept.
// Returns CodeBridgeServiceNotFound if no selector, CodeBridgeUnsupportedProtocol if UDP.
func InspectService(kubeconfig, context, namespace, service string) (*ServiceInfo, error)

// ValidateInterceptable checks that a Service is safe to intercept:
// - Has a non-empty spec.selector (rejects ExternalName and manually-managed Endpoints)
// - All ports are TCP (rejects UDP in 46.2)
// - Is not already intercepted by another session (checks devx-bridge-session annotation)
// - Warns (but allows) if mesh sidecar annotations are detected
func ValidateInterceptable(info *ServiceInfo) error

// PatchServiceSelector replaces the target Service's selector with the agent pod's labels.
// Also sets the annotation devx-bridge-session=<sessionID> for conflict detection.
func PatchServiceSelector(kubeconfig, context, namespace, service string, newSelector map[string]string, sessionID string) (*ServiceState, error)

// RestoreServiceSelector restores the original Service selector and removes the session annotation.
func RestoreServiceSelector(kubeconfig, context string, state *ServiceState) error

// GetServiceSelector returns the current selector of a Service.
func GetServiceSelector(kubeconfig, context, namespace, service string) (map[string]string, error)
```

#### [MODIFY] [internal/bridge/session.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/bridge/session.go)

Extend the session schema to support intercept entries:

```go
// InterceptEntry represents an active traffic intercept.
type InterceptEntry struct {
    Service          string            `json:"service"`
    Namespace        string            `json:"namespace"`
    TargetPort       int               `json:"target_port"`
    LocalPort        int               `json:"local_port"`
    Mode             string            `json:"mode"`              // "steal" or "mirror"
    AgentPod         string            `json:"agent_pod"`
    SessionID        string            `json:"session_id"`
    OriginalSelector map[string]string `json:"original_selector"` // for restore
    StartedAt        time.Time         `json:"started_at"`
}

// Session (extended)
type Session struct {
    Kubeconfig string            `json:"kubeconfig"`
    Context    string            `json:"context"`
    Entries    []SessionEntry    `json:"entries"`
    Intercepts []InterceptEntry  `json:"intercepts,omitempty"` // NEW
    StartedAt  time.Time         `json:"started_at"`
}
```

#### [MODIFY] [internal/devxerr/error.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/devxerr/error.go)

Add new exit codes:

```go
// Bridge Intercept Errors (Idea 46.2)
CodeBridgeAgentDeployFailed     = 65 // Agent Job failed to deploy or reach Running state
CodeBridgeAgentHealthFailed     = 66 // Agent /healthz did not return 200 within timeout
CodeBridgeSelectorPatchFailed   = 67 // Failed to patch Service selector
CodeBridgeRBACInsufficient      = 68 // Insufficient RBAC permissions for intercept
CodeBridgeInterceptActive       = 69 // Another intercept is already active for this service
CodeBridgeUnsupportedProtocol   = 70 // Target port uses UDP (not supported in 46.2)
CodeBridgeTunnelFailed          = 71 // Yamux tunnel failed to establish or dropped
CodeBridgeServiceNotInterceptable = 72 // Service has no selector or is ExternalName type
```

---

### Safety Mechanisms

| Mechanism | Purpose |
|-----------|---------|
| **Self-healing agent** | Agent monitors Yamux connection health; on disconnect or SIGTERM, **automatically restores the original Service selector** before exiting — no manual intervention needed |
| **Dedicated ServiceAccount** | Agent runs with a narrow RBAC role scoped to `update` on the specific target Service only — no cluster-wide permissions |
| **Yamux heartbeat** | Agent sends keepalives every 10s; if no response within 30s, triggers self-healing |
| **`activeDeadlineSeconds: 14400`** | Agent Job auto-terminates after 4 hours as a last-resort safety net (self-healing fires first) |
| **`ttlSecondsAfterFinished: 60`** | Completed/failed Jobs are garbage-collected within 1 minute |
| **Session file backup** | Original Service selector is persisted to `bridge.json` before patching |
| **`devx bridge disconnect --force`** | Emergency restore of all patched selectors from session metadata, even without an active session |
| **Agent labels** | All agent resources are labeled `devx-bridge=agent` for easy discovery/cleanup |
| **Ctrl+C handler** | CLI-side graceful shutdown restores the selector before exiting (belt-and-suspenders with agent self-healing) |
| **Dynamic port mirroring** | Agent Pod spec mirrors target Service's named ports, preventing Endpoint registration failures |

---

### Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| Client crashes mid-intercept (`SIGKILL`, laptop dies) | Agent detects Yamux tunnel drop via heartbeat timeout (30s), **automatically restores Service selector**, then exits. No orphaned state. |
| `activeDeadlineSeconds` expires (4h) | Agent receives `SIGTERM` → restores selector → exits. Job is garbage-collected via `ttlSecondsAfterFinished`. |
| Two developers intercept same service | First-writer-wins: the selector patch stores a `devx-bridge-session` annotation on the Service. Second attempt detects the annotation and exits with `CodeBridgeInterceptActive`. |
| Target Service has no selector (ExternalName) | Exit with `CodeBridgeServiceNotInterceptable` and message "Service has no selector — cannot intercept ExternalName services" |
| Target Service has `protocol: UDP` ports | Exit with `CodeBridgeUnsupportedProtocol` and message "UDP port interception is not supported — only TCP services can be intercepted" |
| Multi-port Service (e.g., `8080` + `9090` metrics) | Agent Pod spec includes ALL ports from the Service. Intercepted port is tunneled to local; non-intercepted ports return `connection refused` (acceptable trade-off — documented) |
| Named `targetPort` (e.g., `http-api`) | Agent Pod spec mirrors the named `containerPort` so the Endpoints controller can resolve it correctly |
| RBAC insufficient for creating Jobs | Exit with `CodeBridgeRBACInsufficient` and print the output of `devx bridge rbac` for the admin to apply |
| Agent image pull fails (air-gapped cluster) | Exit with `CodeBridgeAgentDeployFailed` and suggest using `--agent-image` flag to specify a pre-loaded image |
| Service mesh (Istio/Linkerd) detected | Print warning: "Service mesh detected — intercept may not work as expected. See docs/guide/bridge.md#service-mesh" |
| Cluster uses NetworkPolicy blocking agent | Agent's `/healthz` check times out → `CodeBridgeAgentHealthFailed` with hint about NetworkPolicy |
| Yamux tunnel drops but agent is still running | Agent self-heals. `devx` CLI detects tunnel loss, prints error, exits cleanly. |

---

### Documentation Updates

#### [MODIFY] [docs/guide/bridge.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/bridge.md)

Major expansion:
- New "Traffic Interception" section with architecture diagram
- RBAC requirements section with example ClusterRole YAML
- Service mesh limitations section
- `devx bridge intercept` command reference
- `devx bridge rbac` command reference
- Safety mechanisms documentation

#### [MODIFY] [devx.yaml.example](file:///Users/james/Workspace/gh/application/vitruvian/devx/devx.yaml.example)

Add intercept configuration example under bridge section.

#### [MODIFY] [FEATURES.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/FEATURES.md)

Add Idea 46.2 entry after shipping.

#### [MODIFY] [IDEAS.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/IDEAS.md)

Mark 46.2 status.

#### [MODIFY] [docs/guide/architecture.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/docs/guide/architecture.md)

Update Bridge Layer section with intercept architecture.

#### [MODIFY] [.agent/skills/devx/SKILL.md](file:///Users/james/Workspace/gh/application/vitruvian/devx/.agent/skills/devx/SKILL.md) + all templates

Add intercept commands, exit codes, RBAC requirements.

---

## Gap Analysis

| Area | Current State (46.1) | Gap for 46.2 | Resolution |
|------|---------------------|--------------|------------|
| **Data plane tunnel** | `kubectl port-forward` (local→pod, unidirectional) | Need cluster→local traffic routing (bidirectional) | Yamux multiplexed tunnel over `kubectl port-forward` (new `internal/bridge/tunnel.go`) |
| **Cluster write access** | Read-only (`get svc`, port-forward) | Needs `create/delete jobs/serviceaccounts/roles/rolebindings`, `update svc` | RBAC documentation + `devx bridge rbac` helper + per-agent ServiceAccount |
| **Crash recovery** | N/A — port-forwards are stateless | Service selector left patched if client crashes | Self-healing agent: restores selector on tunnel drop or SIGTERM via dedicated ServiceAccount |
| **Named/multi-port services** | N/A | Agent Pod must mirror target's `containerPorts` (names + numbers) for Endpoints resolution | Dynamic Pod spec generation in `DeployAgent()` |
| **Protocol validation** | N/A | UDP services cannot be tunneled over Yamux/TCP | `ValidateInterceptable()` rejects UDP with `CodeBridgeUnsupportedProtocol` |
| **Service type validation** | Partial (ExternalName mentioned in edge cases) | ExternalName and selector-less services silently fail | First-class `ValidateInterceptable()` check with `CodeBridgeServiceNotInterceptable` |
| **Agent image** | N/A — pure kubectl subprocess | Need a container image published to a registry | Build in separate Go module, publish to GHCR via CI |
| **Service patching** | N/A | Must swap/restore Service selectors atomically | Store original selector in session file + agent env var; dual restore path (CLI + agent self-healing) |
| **Session schema** | `SessionEntry` for port-forwards only | Need `InterceptEntry` with mode, agent pod, original selector | Extend `Session` struct with `Intercepts` field |
| **Error codes** | 60–64 (bridge connect) | Need 65–72 (intercept-specific) | Add to `devxerr` |
| **`bridge disconnect`** | Only kills port-forward processes | Must also delete agent Jobs + RBAC resources and restore selectors | Extend teardown logic |
| **Multi-developer safety** | N/A — port-forwards are local-only | Two devs intercepting same service = data loss | Session annotation on Service + `CodeBridgeInterceptActive` |
| **Go dependency** | No external deps for bridge | Need `hashicorp/yamux` for multiplexing | Add to `go.mod` (CLI side); agent module has its own `go.mod` |
| **Template sync** | Updated in 46.1 fix | Must sync SKILL.md changes to all 5 template dirs | Follow established pattern |

---

## devx.yaml Schema Extension

```yaml
bridge:
  kubeconfig: ~/.kube/config
  context: gke_my-org_us-central1_staging
  namespace: default
  agent_image: ""  # optional: override default ghcr.io/VitruvianSoftware/devx-bridge-agent

  # Outbound connectivity (Idea 46.1)
  targets:
    - service: redis
      port: 6379

  # Inbound traffic interception (Idea 46.2)
  intercepts:
    - service: payments-api
      port: 8080
      mode: steal        # required: "steal" or "mirror" (mirror: future 46.2b)
      local_port: 8080   # optional: local port to route traffic to
```

Corresponding Go schema addition to `devxconfig.go`:

```go
// DevxConfigBridgeIntercept defines an inbound traffic intercept target.
type DevxConfigBridgeIntercept struct {
    Service   string `yaml:"service"`    // K8s service to intercept
    Namespace string `yaml:"namespace"`  // Override namespace (default: bridge.namespace)
    Port      int    `yaml:"port"`       // Remote service port to intercept
    LocalPort int    `yaml:"local_port"` // Local port to route traffic to (default: same as port)
    Mode      string `yaml:"mode"`       // "steal" or "mirror" (required)
}

// DevxConfigBridge (extended)
type DevxConfigBridge struct {
    Kubeconfig string                      `yaml:"kubeconfig"`
    Context    string                      `yaml:"context"`
    Namespace  string                      `yaml:"namespace"`
    AgentImage string                      `yaml:"agent_image"` // NEW: override agent image
    Targets    []DevxConfigBridgeTarget    `yaml:"targets"`     // Outbound (46.1)
    Intercepts []DevxConfigBridgeIntercept `yaml:"intercepts"`  // Inbound (46.2)
}
```

---

## Verification Plan

### Automated Tests

```bash
# Unit tests for new internal/bridge/ files
go test ./internal/bridge/... -v -count=1

# Full test suite
devx action ci
```

#### Unit Tests to Write
- `internal/bridge/agent_test.go` — Test dynamic Pod spec generation (named ports, multi-port), label construction, deadline defaults, ServiceAccount/Role YAML generation
- `internal/bridge/intercept_test.go` — Test `InspectService` parsing, `ValidateInterceptable` (ExternalName rejection, UDP rejection, empty selector rejection, mesh detection warning), selector swap/restore logic, session serialization with intercept entries
- `internal/bridge/tunnel_test.go` — Test Yamux session establishment, stream proxying, heartbeat timeout detection
- `cmd/bridge_intercept_test.go` — Test CLI flag parsing (explicit `--steal` requirement, mode error without flag), dry-run output, `--agent-image` override resolution

### Integration Testing (Manual — requires a live cluster)

```bash
# 1. Spawn a local k3s cluster for integration testing
devx k8s spawn test-intercept --json -y

# 2. Deploy an echo service with NAMED PORTS (tests dynamic port mirroring)
kubectl --kubeconfig ~/.kube/devx-test-intercept.yaml apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo
  labels:
    app: echo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: echo
  template:
    metadata:
      labels:
        app: echo
    spec:
      containers:
      - name: echo
        image: hashicorp/http-echo:latest
        args: ["-text=original"]
        ports:
        - name: http-api
          containerPort: 5678
---
apiVersion: v1
kind: Service
metadata:
  name: echo
spec:
  selector:
    app: echo
  ports:
  - name: http-api
    port: 5678
    targetPort: http-api   # NAMED targetPort — validates dynamic port mirroring
    protocol: TCP
EOF

# 3. Verify original service works
kubectl port-forward svc/echo 5678:5678 &
curl localhost:5678  # Should return "original"
kill %1

# 4. Run intercept
devx bridge intercept echo --steal --port 5678 --local-port 5678 \
  --kubeconfig ~/.kube/devx-test-intercept.yaml

# 5. In another terminal: start local service on port 5678
python3 -m http.server 5678  # or any local server

# 6. From inside the cluster, verify traffic reaches local:
kubectl exec -it <some-other-pod> -- curl echo:5678
# Should return response from local python server, NOT "original"

# 7. Ctrl+C the intercept
# Verify: Service selector is restored (kubectl get svc echo -o jsonpath='{.spec.selector}')
# Verify: Agent Job is deleted (kubectl get jobs -l devx-bridge=agent)
# Verify: ServiceAccount/Role/RoleBinding cleaned up
# Verify: bridge.json intercepts array is empty

# 8. Crash recovery test
devx bridge intercept echo --steal --port 5678 --local-port 5678 \
  --kubeconfig ~/.kube/devx-test-intercept.yaml &
DEVX_PID=$!
sleep 5
kill -9 $DEVX_PID  # Simulate hard crash
# Wait 30s for agent heartbeat timeout
sleep 35
# Verify: Agent has auto-restored the Service selector
kubectl get svc echo -o jsonpath='{.spec.selector}' # Should show {"app":"echo"}

# 9. Cleanup
devx k8s rm test-intercept -y
```

### Edge Case Scenarios to Verify
| Scenario | Test |
|----------|------|
| Client crash (`SIGKILL`) | Kill `devx` PID during intercept; wait 35s → agent should self-heal and restore selector |
| `activeDeadlineSeconds` expiry | Set `--deadline=60` (test flag); wait 60s → agent should restore selector and exit |
| `--dry-run` | Must show all planned actions (including dynamic port spec) without modifying the cluster |
| `--json` | Must output structured JSON suitable for AI agents |
| No mode flag | `devx bridge intercept echo` without `--steal` → should exit with mode required error |
| RBAC insufficient | Use a restricted kubeconfig → should exit `68` with actionable guidance |
| Double intercept | Two terminals intercepting same service → second should exit `69` |
| Graceful Ctrl+C | Must restore selector and delete Job + RBAC resources before exiting |
| ExternalName Service | Attempt intercept → should exit `72` with clear message |
| UDP port Service | Attempt intercept → should exit `70` with clear message |
| Named `targetPort` | Deploy service with `targetPort: http-api` → intercept should work (agent mirrors port name) |

### Build Validation

```bash
go vet ./...
go build ./...
go test ./... -count=1
```
