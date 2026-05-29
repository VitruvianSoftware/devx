# Design: Host workspace mounts for multi-node Lima VMs

**Date:** 2026-05-28
**Status:** Approved (design); pending implementation plan
**Topic:** Make `devx cluster` provision a `mounts:` block on its Lima VMs (defaulting to the host home dir) so devcontainers and Docker bind mounts running against the VM-hosted daemon can see host source files, and let `devx cluster apply` reconcile that mount onto existing VMs.

## Motivation

A user ran `devcontainer exec --workspace-folder . bash` against a devx-provisioned
cluster VM (`k8s-node`, vmType `vz`) and found the container could not see their
macOS source tree. The bind mount existed in `docker inspect` but resolved to an
empty, VM-local directory.

Root cause: the multi-node Lima config generator
([internal/multinode/lima/lima.go:103](../../../internal/multinode/lima/lima.go)
`GenerateConfig`) emits a `lima.yaml` with **no `mounts:` section** — only a
`portForwards` entry for `docker.sock` when Docker is enabled
([lima.go:117-122](../../../internal/multinode/lima/lima.go)). The Docker daemon
runs *inside* the VM, so when `devcontainer`/`docker` asks for a bind mount of
`/Users/<user>/.../repo`, the daemon resolves that path against the **VM's** root
filesystem, where it does not exist. Docker silently creates an empty directory
there and mounts that. The host's files are never shared into the VM, so nothing
the daemon launches can see them.

The legacy single-VM provider
([internal/provider/lima.go:38](../../../internal/provider/lima.go) `Init`) does
not have this problem — it passes `--mount-writable`, so Lima shares the home dir.
Only the multi-node cluster path is affected. The user's `k8s-node` VM was created
by that path (vmType `vz`, docker socket forward, no mounts), which matches the
observed broken state exactly.

## Goals

- New multi-node VMs are provisioned with a `mounts:` block so devcontainers and
  Docker bind mounts against the VM daemon see host source files transparently.
- The mount set is configurable in `cluster.yaml`, defaulting to the host home dir
  so users get a working setup with zero configuration.
- `devx cluster apply` brings **existing** VMs (like the user's running `k8s-node`)
  up to the configured mounts, with an explicit confirmation because it restarts
  the VM and briefly disrupts Kubernetes.

## Non-goals (YAGNI)

- No change to the legacy single-VM provider
  ([internal/provider/lima.go](../../../internal/provider/lima.go)) — it already
  shares home via `--mount-writable`.
- No change to how `devx shell` mounts the workspace into a container
  ([cmd/shell.go](../../../cmd/shell.go)).
- No per-node mount overrides initially. Mounts are cluster-wide. (Can be added
  later if a real need appears.)
- No attempt to wrap or generate `.devcontainer/devcontainer.json`.

## Design

### 1. Config schema

Add a cluster-wide `mounts` field to `ClusterConfig`
([internal/multinode/config/config.go:40](../../../internal/multinode/config/config.go)):

```go
type ClusterConfig struct {
    // ...existing fields...
    Mounts []MountConfig `yaml:"mounts,omitempty"`
}

// MountConfig describes a host directory shared into the cluster VMs.
type MountConfig struct {
    Location   string `yaml:"location"`              // host path; "~" allowed and expanded on the target host
    MountPoint string `yaml:"mountPoint,omitempty"`  // guest path; defaults to the same path as Location
    Writable   bool   `yaml:"writable,omitempty"`    // default false; the injected home-default sets this true
}
```

### 2. Default-vs-opt-out semantics

The fix must work with no configuration, but a user must also be able to turn
mounts off. `gopkg.in/yaml.v3` unmarshals an **omitted** key to a `nil` slice and
an **explicitly empty** key (`mounts: []`) to a non-nil, zero-length slice. We use
that distinction in a single helper:

```go
// resolveMounts returns the effective mount set for a node.
//   nil   (key omitted)      -> default: [{Location: "~", Writable: true}]
//   []    (mounts: [])       -> opt out: no mounts
//   [...] (explicit entries) -> used as-is
func resolveMounts(m []config.MountConfig) []config.MountConfig
```

This helper is the single source of truth and is used by **both** provisioning and
apply so the two paths can never diverge.

### 3. Why `location: "~"` (not a resolved absolute path)

The default emits the literal `location: "~"`, not an `os.Getenv("HOME")`-resolved
path. Two reasons:

- **Same-path guest mapping.** Lima expands `~` on the *target host* and mounts it
  at the *same absolute path* inside the guest (e.g. host `/Users/james` → guest
  `/Users/james`). That identical path is precisely what makes a Docker bind mount
  of `/Users/james/.../repo` resolve correctly inside the VM. This is the whole fix.
- **Host portability.** Multi-node provisioning runs `limactl` on remote hosts over
  SSH ([internal/multinode/lima/lima.go:143](../../../internal/multinode/lima/lima.go)
  `Provision`, via `remote.Runner`). Emitting `~` lets each host expand its own home,
  rather than baking the machine-running-devx's home path into every node.

### 4. Lima config generation

Extend `GenerateConfig`
([internal/multinode/lima/lima.go:103](../../../internal/multinode/lima/lima.go))
to take the resolved mounts and render:

- a top-level `mountType: "virtiofs"` (correct and fast for vmType `vz`), and
- a `mounts:` block with one entry per mount. Emit `mountPoint` only when it
  differs from `location`; emit `writable: true` only when set.

Signature becomes `GenerateConfig(socketPath string, dockerEnabled bool, mounts []config.MountConfig) string`.
`Provision` ([lima.go:143](../../../internal/multinode/lima/lima.go)) gains a
`mounts` parameter and passes the resolved set through. The mounts are threaded
from `ClusterConfig` to the node managers at the `Provision` call site in
([internal/multinode/cluster/cluster.go:87](../../../internal/multinode/cluster/cluster.go),
currently called with just `dockerEnabled`).

Rendered example (default case):

```yaml
mountType: "virtiofs"
mounts:
  - location: "~"
    writable: true
```

### 5. Reconcile onto existing VMs (`devx cluster apply`)

`Apply` ([internal/multinode/cluster/apply.go:37](../../../internal/multinode/cluster/apply.go))
today does **not** truly diff — it unconditionally drains, stops, `sed`-patches
cpu/memory/disk, and restarts every node. For mounts we add a **real diff** so we
only restart when the mount set is actually wrong:

1. Read the live `~/.lima/<vm>/lima.yaml` on the node (over the existing
   `remote.Runner`) and parse its `mounts:` / `mountType:`.
2. Compare to the resolved desired mounts. If equal, **skip** (no restart).
3. If different or missing:
   a. **Confirm** with the user before doing anything destructive — a mount change
      restarts the VM and briefly disrupts Kubernetes on that node. Reuse the
      interactive `fmt.Scanln` "y" pattern already used in
      ([internal/multinode/cluster/cluster.go:440](../../../internal/multinode/cluster/cluster.go)).
      Honor the global `DryRun` flag (print the planned change, change nothing) and
      add a `-y/--non-interactive` flag to `apply` mirroring `destroy`
      ([cmd/cluster_mgmt_destroy.go:40](../../../cmd/cluster_mgmt_destroy.go)).
   b. Drain the node, stop the VM.
   c. **Rewrite the `mounts:`/`mountType:` block in `~/.lima/<vm>/lima.yaml`
      directly**, consistent with the existing file-edit approach apply uses for
      cpu/memory ([apply.go:92](../../../internal/multinode/cluster/apply.go)).
      Chosen over `limactl edit --set` because `--set`'s accepted syntax is
      version-fragile and merge/dedup of a list field is awkward; a deterministic
      block rewrite is robust and idempotent.
   d. Restart the VM, wait, uncordon.

### 6. Validation

Extend `Validate` ([internal/multinode/config/validate.go:29](../../../internal/multinode/config/validate.go))
to reject any mount entry with an empty `location`.

## Testing

- `GenerateConfig` (pure string rendering — easy to assert):
  - omitted mounts → renders the default `~` writable mount + `mountType: virtiofs`.
  - explicit custom mounts → rendered verbatim, with `mountPoint` only when it differs.
  - `mounts: []` → renders **no** `mounts:` block.
- `resolveMounts` unit tests for the three nil/empty/explicit cases.
- `Validate` rejects an empty `location`.
- Mount-diff detection in apply: extend
  ([internal/multinode/cluster/cluster_reconcile_test.go](../../../internal/multinode/cluster/cluster_reconcile_test.go))
  to assert "equal → no restart" and "different → restart (gated on confirm/dry-run)".

## Docs

- Document the `mounts:` block, the `~`/home default, and the `mounts: []` opt-out
  in [cluster.yaml.example](../../../cluster.yaml.example).
- Add a `CHANGELOG.md` entry and note the capability in `FEATURES.md`.

## Migration note for the reported case

The user's existing `k8s-node` VM has no mounts. After this change they would set
(or accept the default) `mounts:` in `cluster.yaml` and run `devx cluster apply`,
which will confirm, then restart that VM with the home mount — bouncing the local
k8s cluster once. Alternatively a destroy + re-provision picks up the mount with no
diff logic involved.
