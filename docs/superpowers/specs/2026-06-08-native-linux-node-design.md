# Design: Native Linux host nodes (`kind: native`) for the multi-node cluster

- **Status:** Approved (design), implementation pending
- **Date:** 2026-06-08
- **Author:** James H. Nguyen (with Claude)
- **Topic slug:** `native-linux-node`

## 1. Goal

Today `devx cluster` provisions every node the same way: SSH into a **macOS** host,
create a **Lima VM**, and install K3s *inside the VM*. This adds a second host
type — a **native Linux host** that already runs Linux, so K3s installs **directly
on the host** (no VM). The motivating host is a **Fedora Silverblue 44** box on the
operator's tailnet, to be added as a worker (and, later, a potential replacement
for a flaky Mac control-plane node).

## 2. Decisions (from brainstorming)

- **Scope:** a general **native Linux** host type (works for Fedora/Ubuntu/etc.),
  with Silverblue's immutability/SELinux handled — not a one-off.
- **Role:** **agent by default**, but the native provider must also support
  **server** (to replace an unreliable Mac control-plane node; a *replacement*
  keeps the server count odd, so it does not trip the even-count guard).
- **Architecture:** a full **`NodeProvider` interface** (Approach A) — both host
  types implement it; `cluster.go` routes through it. Cleaner long-term than
  inline `kind` branches, at the cost of a behavior-preserving Lima refactor.

## 3. Host grounding (probed `fedora` = `100.97.82.15`)

- Fedora **Silverblue 44** (rpm-ostree immutable, idle), x86_64, 24 cores / 62 GB.
- SELinux **Permissive**; **passwordless sudo**; `/usr/local/bin` writable via sudo
  (K3s installs there and survives OS updates). cgroup **v2**.
- `tailscale` up (`100.97.82.15`), `podman`, `curl`, `rpm-ostree`, `firewall-cmd`
  present; egress to `get.k3s.io` works; no K3s yet.
- **firewalld active** (`FedoraWorkstation` zone); **`tailscale0` is not trusted**.
- **`conntrack` and `socat` are missing** (both needed: conntrack by kube-proxy,
  socat by `kubectl port-forward` / `devx bridge`).

## 4. Architecture

A new `internal/multinode/provider` package defines a `NodeProvider` interface
implemented by both host types, selected per-node by a `kind` field.

```go
// internal/multinode/provider/provider.go
type NodeProvider interface {
    EnsureRuntime(ctx context.Context) (nodeIP string, err error)
    InstallServer(ctx context.Context, o JoinOpts) error
    InstallAgent(ctx context.Context, o JoinOpts) error
    GetToken(ctx context.Context) (string, error)
    IsInstalled(ctx context.Context) (bool, error)
    WaitForReady(ctx context.Context, timeout time.Duration) error
    NodeStatus(ctx context.Context) (string, error)         // kubectl get nodes (from a server)
    Kubeconfig(ctx context.Context, serverIP string) (string, error)
    Drain(ctx context.Context, nodeName string) error
    DeleteNode(ctx context.Context, nodeName string) error
    Uninstall(ctx context.Context, role string) error
    Destroy(ctx context.Context) error                      // native: == Uninstall (no VM)
    Reconcile(ctx context.Context) error
}

// JoinOpts carries the recipe inputs shared by both providers.
type JoinOpts struct {
    NodeIP, ServerURL, Token, Pool, K3sVersion string
    TLSSANs                                     []string
    DisableServiceLB, UseTailscale, UseDocker   bool
}

func For(node config.NodeConfig, cfg *config.Config) NodeProvider // factory by node.Kind
```

- **`LimaProvider`** — a thin, **behavior-preserving** wrapper over the existing
  `lima.Manager` + `k3s.Manager`. `EnsureRuntime` provisions the VM and returns its
  bridged/tailscale IP; the rest delegate to the existing managers (which run via
  `limactl shell`). No change to existing macOS behavior.
- **`NativeProvider`** — new (Section 5).
- `cluster.go` Init/Join/Remove/Destroy/Status/Reconcile call `provider.For(node)`
  methods instead of `lima`/`k3s` directly. The K3s *recipe* (env, flags, token,
  server URL) is unchanged — only the execution wrapper and host prep differ.

## 5. NativeProvider behavior (Fedora/Silverblue)

Runs everything over the existing SSH `remote.Runner` (`ssh host sudo sh -c …`),
not `limactl shell`.

- **`EnsureRuntime`:**
  1. verify SSH + `sudo -n`;
  2. **install deps** via detected package manager — `rpm-ostree install
     --apply-live --allow-inactive --idempotent conntrack-tools socat` on
     rpm-ostree hosts, else `dnf install -y` / `apt-get install -y`;
  3. **trust the tailnet**: if firewalld active, `firewall-cmd --permanent
     --zone=trusted --add-interface=tailscale0 && firewall-cmd --reload`;
  4. ensure Tailscale is up (else `tailscale up --authkey=… --accept-routes`);
  5. return the node-IP: `tailscale ip -4` (or `node.NodeIP` override).
- **`InstallServer` / `InstallAgent`:** the **same `get.k3s.io` recipe the Lima
  path uses**, run via SSH sudo, with `--node-ip=<ip> --flannel-iface=tailscale0`,
  `--node-label=pool=<pool>`, and `INSTALL_K3S_SKIP_SELINUX_RPM=true` (Permissive).
  Both roles supported.
- **`GetToken` / `IsInstalled` / `WaitForReady` / `NodeStatus` / `Kubeconfig` /
  `Drain` / `DeleteNode` / `Uninstall`:** the same `k3s …` / `k3s kubectl …`
  commands the Lima provider runs, but directly on the host.
- **`Destroy`:** uninstall K3s (`/usr/local/bin/k3s-*-uninstall.sh`); there is no
  VM to remove. Best-effort: untrust `tailscale0`, leave deps in place.
- **`Reconcile`:** ensure the node baseline (conntrack/socat) via the same
  package-manager path as `EnsureRuntime`.

## 6. Config & validation

- `NodeConfig.Kind string` (`yaml:"kind"`, default `"lima"`; `lima` | `native`).
- `NodeConfig.NodeIP string` (`yaml:"nodeIP,omitempty"`) — optional override for
  native nodes; default is the auto-detected `tailscale ip`.
- VM fields (`vm.cpus/memory/disk`) are **required only for `kind: lima`**;
  validation skips them for `native`.
- `kind` is validated against the known set; the even-server-count rule is
  unchanged.

```yaml
nodes:
  - host: fedora        # SSH target (tailnet name or IP)
    kind: native
    role: agent
    pool: linux
```

## 7. Components & layout

- `internal/multinode/provider/provider.go` — interface, `JoinOpts`, `For` factory.
- `internal/multinode/provider/lima.go` — `LimaProvider` (wraps existing managers).
- `internal/multinode/provider/native.go` — `NativeProvider` + package-manager
  detection + firewalld/tailscale/k3s command construction (injectable runner).
- `internal/multinode/config` — add `Kind`/`NodeIP`; relax VM validation for native.
- `internal/multinode/cluster/cluster.go` — route Init/Join/Remove/Destroy/Status/
  Reconcile through `provider.For(node)`.

## 8. Testing

- **Native (unit, injectable runner):** golden/table tests for the generated K3s
  install (agent + server), the `firewall-cmd … --add-interface=tailscale0`, and
  the dep-install command per package manager (rpm-ostree `--apply-live` / dnf /
  apt). Package-manager detection from `/etc/os-release` + tool presence.
- **Lima provider:** behavior-preserving extraction — existing cluster tests pass
  unchanged.
- **Factory:** `kind → provider` selection (incl. default/empty → lima, invalid → error).
- **Gated integration** (opt-in, like the USB work): join `fedora` as an agent,
  assert it reaches `Ready` in the cluster, then remove it.

## 9. Out of scope / field-validation

- SELinux **Enforcing** (layer `k3s-selinux` via rpm-ostree + reboot) — deferred;
  detect Permissive now, warn on Enforcing.
- `rpm-ostree --apply-live` unsupported on older rpm-ostree → reboot fallback (noted).
- **Tailscale ACLs** must allow `fedora ↔ cluster VMs` — the join URL is the server
  **VM's** tailnet IP (from coords-resolution); reachability is a field check.
- Non-Linux native hosts (Windows, etc.) — out.
- Wi-Fi/Ethernet selection on the host — uses the host's existing routing.
