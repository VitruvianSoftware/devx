# Idea 46: Hybrid Edge-to-Local Cloud Routing (`devx bridge`)

> **Status: ✅ APPROVED — Ready for Execution**
> All design decisions have been reviewed and approved by the product owner.

## Background

Complex bugs sometimes only manifest with real staging integration data. Running all 50 microservices locally is impossible, but testing a local fix against a remote cluster is tedious. Idea 46 proposes a Telepresence-like `devx bridge` that securely intercepts traffic from a specific microservice in a remote K8s staging environment and tunnels it to the developer's local `devx` container.

### The Design Tension

The PRODUCT_ANALYSIS.md codifies a **"Client-Side Only Architecture"** principle:

> No bloated centralized SaaS proxy servers or massive Kubernetes cluster controllers required. `devx` runs completely locally.

The IDEAS.md entry for Idea 46 explicitly notes this tension:

> "Traffic interception requires either a cluster-side agent (breaking the 'Client-Side Only' design principle) or DNS-level routing tricks."

**After extensive research**, the technical reality is clear: truly client-side-only traffic *interception* from a Kubernetes cluster is not possible. Traffic must be rerouted **before** it reaches the original pod, which requires a temporary in-cluster component. However, the industry has moved toward **ephemeral, auto-cleaning agents** (pioneered by [mirrord](https://metalbear.co/mirrord/)) that are semantically closer to "client-driven" than "cluster-installed."

> [!IMPORTANT]
> **Design Decision: "Client-Driven, Cluster-Ephemeral" — ✅ APPROVED**
>
> We are amending the "Client-Side Only" principle to **"Client-Driven Architecture"** — meaning `devx` never requires pre-installed cluster-side infrastructure or permanent agents. Instead, `devx bridge` deploys a **short-lived ephemeral agent pod** that auto-cleans on session end, controlled entirely from the client. This is the same architectural approach used by mirrord (the current industry leader).
>
> **Action Required:** The `PRODUCT_ANALYSIS.md` design principles section and all relevant documentation (`docs/guide/introduction.md`, `docs/guide/architecture.md`) must be updated to reflect this "Client-Driven" wording, replacing the stricter "Client-Side Only" language where appropriate.

---

## Resolved Decisions (From User Review)

> [!TIP]
> **Architectural Deviation — ✅ Approved**
>
> The mirrord-style ephemeral approach is approved. This is the first `devx` command that modifies state in a remote Kubernetes cluster (deploying an ephemeral pod in Phase 46.2+). Phase 46.1 is purely client-side (`kubectl port-forward`).

> [!TIP]
> **Scope: Phased Delivery as Idea 46.1 / 46.2 / 46.3 — ✅ Approved**
>
> Each phase is managed as a separate trackable idea to keep scope contained. Idea 46.1 ships independently with full value. 46.2 and 46.3 are future enhancements.

> [!TIP]
> **`kubectl` as a new dependency — ✅ Accepted**
>
> Most developers targeting this feature already have `kubectl` installed since they are deploying and operating their applications in Kubernetes. Added as a conditional dependency in `devx doctor` (required only when `bridge:` is configured in `devx.yaml`).

---

## Proposed Changes

### Idea 46.1: Outbound Bridge — "Connect Local to Cluster" (MVP)

**Goal:** Allow a local process to access remote Kubernetes services as if it were inside the cluster. No traffic interception yet — just outbound connectivity.

**How it works:**
1. Developer specifies a kubeconfig and target namespace
2. `devx bridge connect` establishes `kubectl port-forward` tunnels to specified remote services
3. Injects `BRIDGE_<SERVICE>_URL` / `BRIDGE_<SERVICE>_HOST` / `BRIDGE_<SERVICE>_PORT` env vars into `~/.devx/bridge.env` for consumption by `devx shell`
4. `devx shell` automatically sources these vars when a bridge session is active

**Value:** Developers can run their local service that calls remote staging APIs using injected env vars — zero config changes to application code.

> [!NOTE]
> **Future Enhancement (Idea 46.1.5): DNS Proxy**
>
> A lightweight Go DNS proxy on `127.0.0.1:15353` that resolves `*.svc.cluster.local` patterns to forwarded local ports (+ `/etc/resolv.conf` modification) would allow apps to use native k8s DNS names without env var changes. This requires `sudo` and risks breaking other tools, so it's deferred. When implementing, use Go's `net` package to build the proxy, listen on a non-privileged high port, and add a `--dns` flag to `devx bridge connect` to opt in. Falls through to system DNS for non-cluster domains.

---

#### Idea 46.1 Components

---

##### `devx.yaml` Schema Extension

###### [MODIFY] [devxconfig.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/devxconfig.go)

Add new schema types for bridge configuration:

```go
// DevxConfigBridgeTarget defines a remote K8s service to bridge locally.
type DevxConfigBridgeTarget struct {
    Service   string `yaml:"service"`   // K8s service name (e.g., "payments-api")
    Namespace string `yaml:"namespace"` // K8s namespace (default: from bridge.namespace)
    Port      int    `yaml:"port"`      // Remote service port to forward
    LocalPort int    `yaml:"local_port"`// Local port to bind (0 = auto)
}

// DevxConfigBridge defines the hybrid edge-to-local routing configuration.
type DevxConfigBridge struct {
    Kubeconfig string                   `yaml:"kubeconfig"` // Path to kubeconfig (default: ~/.kube/config)
    Context    string                   `yaml:"context"`    // Kube context to use
    Namespace  string                   `yaml:"namespace"`  // Default namespace for targets
    Targets    []DevxConfigBridgeTarget `yaml:"targets"`    // Remote services to bridge
}
```

Add `Bridge *DevxConfigBridge` to the `DevxConfig` struct. Add bridge merging logic to `mergeProfile()` and `loadAndResolve()` include resolution.

###### Example `devx.yaml` addition:

```yaml
bridge:
  kubeconfig: ~/.kube/config
  context: gke_my-org_us-central1_staging
  namespace: default
  targets:
    - service: payments-api
      port: 8080
    - service: user-service  
      port: 3000
    - service: redis
      namespace: cache
      port: 6379
```

---

##### CLI Commands

###### [NEW] [bridge.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge.go)

Parent command: `devx bridge` (subcommand group).

```
devx bridge            — Show bridge help
devx bridge connect    — Establish outbound bridge to remote cluster
devx bridge status     — Show active bridge sessions
devx bridge disconnect — Tear down all active bridges
```

###### [NEW] [bridge_connect.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_connect.go)

The core connect command. Responsibilities:
1. Parse `devx.yaml` bridge config (or accept CLI overrides)
2. Validate kubeconfig/context access via `kubectl cluster-info`
3. For each target:
   - Acquire a free local port (reuse `internal/network/ports.go`)
   - Spawn `kubectl port-forward svc/<name> <local>:<remote> -n <ns>` as a managed subprocess
   - Register the forwarded endpoint
4. Inject `BRIDGE_<NAME>_URL` env vars into a bridge env file (`~/.devx/bridge.env`)
6. Display a Bubble Tea TUI showing active bridges with live health indicators
7. Block until Ctrl+C, then gracefully tear down all port-forwards

**CLI flags:**
- `--kubeconfig` — Override kubeconfig path
- `--context` — Override kube context  
- `--namespace` / `-n` — Override default namespace
- `--target` / `-t` — Ad-hoc target (repeatable): `service:port` or `service:port:localport`
- `--json` — Machine-readable output
- `-y` — Non-interactive
- `--dry-run` — Show what would be bridged without connecting

###### [NEW] [bridge_status.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_status.go)

Shows active bridge sessions. Reads bridge state from `~/.devx/bridge.json`. Supports `--json`.

###### [NEW] [bridge_disconnect.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/bridge_disconnect.go)

Tears down all active bridges. Kills port-forward subprocesses, removes DNS proxy, cleans bridge.env. Supports `--dry-run`, `-y`.

---

##### Internal Package

###### [NEW] [internal/bridge/](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/bridge/)

New package containing:

- **`portforward.go`** — Manages `kubectl port-forward` subprocess lifecycle. Wraps `exec.Command`, handles reconnection on transient failures (k8s API server blips), and exposes a health-check channel.
- **`session.go`** — Session state management. Persists active bridge state to `~/.devx/bridge.json` for `bridge status` / `bridge disconnect` / `devx map` consumption.
- **`kube.go`** — Kubeconfig and context validation helpers. Validates cluster reachability, discovers available services/namespaces for TUI selection.
- **`env.go`** — Generates `~/.devx/bridge.env` with `BRIDGE_<SERVICE>_URL` and `BRIDGE_<SERVICE>_HOST`/`BRIDGE_<SERVICE>_PORT` variables for consumption by `devx shell`.

> [!NOTE]
> **Future file: `dns.go`** — Reserved for Idea 46.1.5 (DNS Proxy). Will provide a lightweight DNS proxy using Go's `net` package, listening on `127.0.0.1:15353`, resolving `*.svc.cluster.local` patterns to the correct forwarded local port, falling through to system DNS for everything else.

---

##### Integration with Existing Commands

###### [MODIFY] [shell.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/shell.go)

When a bridge session is active (`~/.devx/bridge.json` exists and has live entries), automatically source `~/.devx/bridge.env` into the container environment. Print a notice: `🔗 Bridge active — injecting BRIDGE_* env vars from staging cluster`.

###### [MODIFY] [map.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/map.go)

When bridge config exists in `devx.yaml`, include remote services in the Mermaid graph with a distinct styling (dashed borders, "☁️ staging" label) to show the local↔remote topology.

###### [MODIFY] [doctor.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/doctor.go)

Add `kubectl` as a conditional prerequisite (required when `bridge:` is present in `devx.yaml`). Check: `kubectl version --client --output=json`.

###### [MODIFY] [nuke.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/cmd/nuke.go)

Include bridge session cleanup (`~/.devx/bridge.json`, `~/.devx/bridge.env`) in the nuke manifest.

---

##### Error Handling & Exit Codes

###### [MODIFY] [error.go](file:///Users/james/Workspace/gh/application/vitruvian/devx/internal/devxerr/error.go)

Add new exit codes:

```go
// Bridge Errors
CodeBridgeKubeconfigNotFound = 60  // kubeconfig file does not exist
CodeBridgeContextUnreachable = 61  // cluster context exists but API server is unreachable
CodeBridgeNamespaceNotFound  = 62  // target namespace does not exist
CodeBridgeServiceNotFound    = 63  // target service does not exist in namespace
CodeBridgePortForwardFailed  = 64  // kubectl port-forward crashed or timed out
```

---

### Idea 46.2: Inbound Interception — "Route Cluster Traffic to Local" (Future)

> [!NOTE]
> Idea 46.2 is **not in scope for the initial implementation.** It is documented here for architectural context and to ensure 46.1 decisions don't paint us into a corner.

**Goal:** Intercept traffic from a remote cluster service and route it to the developer's local container — the core Telepresence/mirrord value proposition.

**How it works:**
1. `devx bridge intercept <service>` — Deploy an ephemeral agent pod in the target service's namespace
2. Agent shares the target pod's network namespace via Kubernetes ephemeral containers API
3. Agent captures incoming TCP traffic and tunnels it back to the local machine via a reverse `kubectl port-forward`
4. Local `devx` proxy receives the traffic and forwards it to the local service container
5. Responses flow back through the same tunnel

**Key decisions for Phase 2:**
- Use Kubernetes Jobs (not Deployments) for the agent — auto-cleanup via `activeDeadlineSeconds`
- Agent image: a minimal Go binary (<10MB) published to `ghcr.io/VitruvianSoftware/devx-bridge-agent`
- Two modes: `--mirror` (duplicate traffic, original service keeps running) and `--intercept` (fully reroute — destructive to other developers)

---

### Idea 46.3: Full Integration — "Seamless Hybrid Topology" (Future)

**Goal:** First-class `devx.yaml` support for hybrid topologies where some services run locally and others are bridged from staging, orchestrated by `devx up`.

```yaml
services:
  - name: api
    runtime: bridge          # NEW runtime type
    bridge:
      context: staging
      service: payments-api
      mode: mirror           # or "intercept"
```

---

## Gap Analysis

| Area | Current State | Gap | Resolution |
|------|--------------|-----|------------|
| **Cluster interaction** | `devx k8s spawn` manages local-only k3s containers | No interaction with remote clusters | 46.1 adds `kubectl port-forward` orchestration |
| **DNS resolution** | N/A — all services are local | Local code can't resolve `*.svc.cluster.local` | 46.1: env vars; 46.1.5: DNS proxy (deferred) |
| **Bridge env injection** | `devx shell` injects vault secrets and AI bridge vars | No bridge endpoint injection | 46.1 extends `shell.go` to source `bridge.env` |
| **`kubectl` dependency** | Not in prerequisite list | Missing from `devx doctor` | 46.1 adds conditional `kubectl` check |
| **Topology visualization** | `devx map` shows local services only | Remote bridged services invisible | 46.1 extends Mermaid graph |
| **Design principle** | "Client-Side Only Architecture" | Bridge requires cluster interaction | Amend to "Client-Driven Architecture" in docs |
| **Documentation** | Design principles say "Client-Side Only" | Stale principle language | Update `PRODUCT_ANALYSIS.md`, `introduction.md`, `architecture.md` |

---

## Resolved Design Decisions

### 1. DNS Strategy — ✅ Env Vars Only (Option B)

**Decision:** Inject `BRIDGE_*_URL` env vars only. No DNS proxy in 46.1.

**Rationale:** Non-invasive, requires no elevated permissions, covers ~90% of use cases.

**Future (Idea 46.1.5 — Option A):** A lightweight Go DNS proxy on `127.0.0.1:15353` that modifies `/etc/resolv.conf` to resolve `*.svc.cluster.local` to forwarded local ports. This requires `sudo` and risks breaking other tools. Deferred until there's organic demand. Implementation notes are captured in the "How it works" section above and in the `internal/bridge/` package note.

### 2. Kubernetes Client Strategy — ✅ kubectl Subprocess (Option B)

**Decision:** Shell out to `kubectl` for all cluster interactions.

**Rationale:** Consistent with `devx`'s established pattern of wrapping external CLIs (`cloudflared`, `podman`, `gh`, `butane`, `mutagen`). Avoids ~40+ transitive Go module dependencies from `client-go`. The `kubectl` binary is already present on most K8s developer machines.

**Rejected alternative (Option A):** Using `k8s.io/client-go` directly would provide more control but adds massive dependency weight and API-version maintenance burden.

---

## Verification Plan

### Automated Tests

```bash
# Unit tests for the new internal/bridge package
go test ./internal/bridge/... -v

# Integration test: bridge connect with a local k3s cluster (Idea 46.1)
# 1. Spawn a local k3s cluster via devx k8s spawn
devx k8s spawn test-bridge --json -y

# 2. Deploy a simple echo service to the local cluster
kubectl --kubeconfig ~/.kube/devx-test-bridge.yaml apply -f /tmp/echo-service.yaml

# 3. Bridge connect to the echo service
devx bridge connect --kubeconfig ~/.kube/devx-test-bridge.yaml --context default --target echo-svc:8080 --json

# 4. Verify the forwarded port is accessible
curl http://localhost:<bridge-port>/health

# 5. Verify BRIDGE_* env vars are generated
cat ~/.devx/bridge.env | grep BRIDGE_ECHO_SVC

# Existing test suite must pass
devx action ci
```

### Edge Case Scenarios

| Scenario | Expected Behavior |
|----------|-------------------|
| Kubeconfig file missing | Exit 60 `CodeBridgeKubeconfigNotFound` with message |
| Cluster unreachable (VPN down) | Exit 61 `CodeBridgeContextUnreachable` with actionable msg |
| Target namespace doesn't exist | Exit 62 `CodeBridgeNamespaceNotFound` |
| Target service doesn't exist | Exit 63 `CodeBridgeServiceNotFound` |
| Port collision on local bind | Auto-shift via `internal/network/ports.go` (consistent with Idea 36) |
| kubectl not installed | `devx doctor` reports it as missing; `bridge connect` fails with `CodeNotLoggedIn`-style error |
| Port-forward drops mid-session | Auto-reconnect with exponential backoff (3 retries, then surface error in TUI) |
| `--dry-run` flag | Print planned bridges without connecting |
| `--json` flag | Output structured JSON of bridge state |
| `-y` flag | Skip kubeconfig/context selection TUI |
| Bridge + `devx shell` | Shell automatically injects `BRIDGE_*` vars |
| Bridge + `devx map` | Remote services appear with dashed borders |

### Build Validation

```bash
go vet ./...
go build ./...
go test ./... -count=1
```

### Documentation Verification

After implementation, verify:
- [ ] `docs/guide/bridge.md` exists and is comprehensive
- [ ] VitePress sidebar in `config.mjs` includes "Hybrid Bridge" entry
- [ ] `devx.yaml.example` includes `bridge:` section with inline comments
- [ ] `IDEAS.md` updated: Idea 46 moved to "Shipped" or "In Progress"
- [ ] `FEATURES.md` updated with Idea 46 entry
- [ ] `README.md` mentions bridge capability
- [ ] `.agent/skills/devx/SKILL.md` updated with bridge section
- [ ] `CONTRIBUTING.md` updated if bridge introduces new patterns

---

## Files Summary

### New Files (Idea 46.1)
| File | Purpose |
|------|---------|
| `cmd/bridge.go` | Parent `devx bridge` subcommand |
| `cmd/bridge_connect.go` | Core `devx bridge connect` implementation |
| `cmd/bridge_status.go` | Bridge session status display |
| `cmd/bridge_disconnect.go` | Bridge teardown |
| `internal/bridge/portforward.go` | kubectl port-forward lifecycle management |
| `internal/bridge/session.go` | Session state persistence (`~/.devx/bridge.json`) |
| `internal/bridge/kube.go` | Kubeconfig/context validation |
| `internal/bridge/env.go` | Bridge env var generation (`~/.devx/bridge.env`) |
| `internal/bridge/portforward_test.go` | Unit tests |
| `internal/bridge/session_test.go` | Unit tests |
| `docs/guide/bridge.md` | Documentation page |

### Modified Files (Idea 46.1)
| File | Change |
|------|--------|
| `cmd/devxconfig.go` | Add `DevxConfigBridge*` schema types |
| `cmd/root.go` | Register `bridgeCmd` |
| `cmd/shell.go` | Source `bridge.env` when active |
| `cmd/map.go` | Render remote bridged services in Mermaid graph |
| `cmd/doctor.go` | Add `kubectl` conditional prerequisite |
| `cmd/nuke.go` | Include bridge cleanup |
| `internal/devxerr/error.go` | Add bridge exit codes (60–64) |
| `devx.yaml.example` | Add `bridge:` example section |
| `docs/.vitepress/config.mjs` | Add sidebar entry |
| `.agent/skills/devx/SKILL.md` | Add bridge section |
| `PRODUCT_ANALYSIS.md` | Amend "Client-Side Only" → "Client-Driven Architecture" |
| `docs/guide/introduction.md` | Reflect "Client-Driven" principle language |
| `docs/guide/architecture.md` | Reflect "Client-Driven" principle language |
