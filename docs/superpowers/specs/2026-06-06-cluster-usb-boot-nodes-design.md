# Design: USB-boot self-joining cluster nodes (`devx cluster usb`)

- **Status:** Approved (core architecture), implementation in progress
- **Date:** 2026-06-06
- **Author:** James H. Nguyen (with Claude)
- **Topic slug:** `cluster-usb-boot-nodes`

## 1. Goal

Today `devx cluster` provisions a K3s cluster (embedded-etcd HA) by SSH-ing into
remote macOS hosts, spinning up a Lima VM on each, and installing K3s inside it.

This feature adds a second **node onboarding path**: produce a **Ventoy multi-boot
USB** such that booting any supported laptop off it, and picking a menu entry,
self-provisions a K3s node that **auto-joins the existing cluster** — over
Tailscale with a LAN fallback, as an **agent/worker** by default.

The join *recipe* is identical to what `devx cluster` already does (the node needs
a server URL, the K3s node-token, and optionally a Tailscale auth key). What is new
is the **delivery mechanism**: instead of pushing the recipe over SSH into a Lima
VM, we bake it into a bootable medium that self-configures on first boot.

## 2. Scope

### In scope
- A `devx cluster usb` command group: `build` (generate artifacts), `prune` (reap
  dead ephemeral nodes), and a gated `prepare` (install Ventoy onto a blank USB).
- Two **lifecycle modes**, selectable at the Ventoy boot menu:
  - **Ephemeral live-boot** — runs from RAM; the laptop's internal disk is *not*
    wiped. Optional **non-destructive scratch storage** (use an existing free
    partition or unallocated space) for containerd state and local-path PVs.
  - **Persistent install** — installs the OS to the internal disk; a permanent
    node that rejoins on reboot.
- Three **renderers** behind one interface (per the "all three now" decision),
  each emitting the same join recipe in a different package:
  - **FCOS** — Fedora CoreOS + Ignition (reuses `internal/ignition`/`butane`).
  - **Ubuntu** — live ISO + cloud-init (NoCloud) / autoinstall; widest Wi-Fi/HW.
  - **Baked** — a prebuilt image + a tiny FAT config partition carrying secrets.
- **Tailscale-preferred / LAN-fallback** join transport, chosen at runtime by the
  node from baked coordinates.
- Security model: separate **K3s agent token** (not the server token), **ephemeral
  tagged Tailscale auth keys**, and a documented lost-stick revocation story.

### Out of scope (this PR) / field-validated only
- Booting real hardware and observing a join — **cannot be verified in CI**; the
  environment has no USB/ISO/laptop/cluster. This PR delivers and unit/golden-tests
  the **generation pipeline**; on-hardware boot is documented as a manual field test.
- Apple Silicon Macs (cannot boot Ventoy). Intel Macs: best-effort.
- ARM laptops (initial cut is `x86_64`).
- Promoting USB nodes to etcd control-plane members (they join as agents).

### Hardware prerequisites (documented for operators)
- `x86_64` UEFI/BIOS laptop.
- **Secure Boot** disabled (Ventoy signed-shim support is best-effort, firmware-dependent).
- **Ethernet recommended** for the FCOS path; the Ubuntu renderer is the
  Wi-Fi-friendly option (SSID/PSK baked into its network config).

## 3. Architecture

Two cleanly separated layers:

1. **The Ventoy stick** is an OS-agnostic multi-boot menu — it can hold any number
   of boot entries; this is cheap and Ventoy-native.
2. **The devx-generated provisioning payload** turns a booted OS into a joined
   node. This is the real engineering surface, and the three renderers differ
   *only* here.

The load-bearing decision: the **join recipe is written once** as a canonical shell
script generated from a `JoinSpec`; renderers only re-package that one script. So
"all three renderers" is 3× the *packaging*, not 3× the *logic*.

```
devx cluster usb build
        │
        ▼
 1. coords.ResolveCoordinates(cfg)         ← reuses cluster's healthy-server + token logic
    → { LANServerURL, TailnetServerURL, AgentToken, K3sVersion }
        │
        ▼
 2. JoinSpec → RenderJoinScript()          ← THE canonical recipe (one source of truth)
    tailscale up (authkey) → pick endpoint (LAN reachable? else tailnet) →
    k3s agent|server install → wire scratch storage → label node (ephemeral=true)
        │
        ▼  same script, different packaging
 ┌───────────────┬────────────────┬──────────────────────────┐
 │ render.FCOS   │ render.Ubuntu  │ render.Baked             │
 │ → Butane/Ign  │ → cloud-init   │ → FAT config partition + │
 │   + live ISO  │   NoCloud /    │   prebuilt-image manifest│
 │   + installer │   autoinstall  │                          │
 └───────────────┴────────────────┴──────────────────────────┘
        │  []BootEntry + []Artifact
        ▼
 3. ventoy.Assemble(entries, artifacts, sink)
    → stage ISOs/payloads + write ventoy.json (menu ↔ injection mapping)
    → output dir (default) or, gated, a real --device
```

### Boot-time data flow (on the laptop, field-tested)
1. Boot off USB → Ventoy menu lists entries (e.g. "Join — ephemeral (FCOS)",
   "Install node to disk (FCOS)", "Join — ephemeral (Ubuntu/Wi-Fi)", …).
2. Pick one → Ventoy boots that ISO with the injected payload.
3. First boot runs the canonical join script: `tailscale up` → endpoint selection
   (try baked LAN URL; else tailnet URL) → `k3s agent` install with token/serverURL
   → mount scratch (existing/free partition) for containerd + local-path → label
   node `devx.io/ephemeral=true` with a unique node name (hostname+MAC+rand).
4. Node appears in `kubectl get nodes`, joined.

### Teardown
- **Ephemeral:** reboot/unplug → node is gone from the OS; the cluster shows a
  `NotReady` ghost → `devx cluster usb prune` drains+deletes `NotReady` ephemeral
  nodes past a TTL (also shippable as an in-cluster CronJob — future).
- **Install:** persists and rejoins on reboot.

## 4. Components & layout (matches existing `internal/multinode/*`)

- `cmd/cluster_mgmt_usb.go` — `newClusterUSBCmd`, sibling of `cluster_mgmt_join.go`;
  registered in `cluster_mgmt.go`. Subcommands `build`, `prune`, `prepare`.
  Honors the `cmd.DryRun` global and a `--json` flag (devx AI-native ethos).
- `internal/multinode/usb/`
  - `joinspec.go` — `JoinSpec`, `Mode`, `Role`, `ScratchConfig`, `RenderJoinScript`.
  - `coords.go` — `ResolveCoordinates(ctx, cfg)`; mirrors the healthy-server +
    `GetToken` logic in `cluster.Join` (kept separate this PR to avoid changing
    `Join`'s behavior; dedupe is a noted follow-up).
  - `build.go` — orchestration (resolve → spec → render → assemble); dry-run plan.
  - `prune.go` — reaper.
  - `render/` — `render.go` (interface + `Mode`/`BootEntry`/`Artifact`/`ArtifactSink`/
    `Registry`), `fcos.go`, `ubuntu.go`, `baked.go`.
  - `ventoy/` — `ventoy.go` (`ventoy.json` generation + staging; gated device write).
- `internal/ignition` — reused; the FCOS renderer adds a k3s/Tailscale/scratch
  Butane template alongside the existing tunnel template (no rewrite).
- `internal/multinode/config` — add an optional `cluster.usb` block (default
  renderers, scratch strategy, node TTL, tailnet server URL, optional Wi-Fi);
  reuse `cluster.tailscale.authKey`.

### Renderer interface (the seam)
```go
type Renderer interface {
    Name() string                 // "fcos" | "ubuntu" | "baked"
    Modes() []Mode                // subset of {Ephemeral, Install}
    Render(spec JoinSpec, m Mode, sink ArtifactSink) ([]BootEntry, error)
}
```
Each `BootEntry` becomes one Ventoy menu line. FCOS and Ubuntu each yield up to two
entries (ephemeral + install); Baked yields one (mode chosen by a flag on its config
partition). A fully-loaded stick shows ~5 entries. Adding a renderer later is purely
additive — Ventoy shows one more line.

## 5. Lifecycle modes & non-destructive scratch storage

`ScratchConfig` strategies (ephemeral mode):
- `none` — pure RAM; smallest footprint.
- `existing:LABEL|UUID|/dev/...` — mount a pre-existing partition the operator
  designates; **never formatted unless `--force-format`**.
- `free-space` — carve a new partition from *unallocated* space only (refuses to
  shrink or touch existing partitions); formatted ephemeral and used for containerd
  data-dir + local-path-provisioner root.

Guardrails: the scratch logic is strictly non-destructive by default — it inspects
the partition table and **bails** rather than risk the operator's OS/data, surfacing
an explicit error the node logs.

## 6. Security model

The stick carries credentials, so:
- Bake a **dedicated K3s agent token** (`--agent-token` on the cluster), *not* the
  server token — a leaked agent token cannot add control-plane servers.
- Use **ephemeral, pre-authorized, ACL-tagged** Tailscale auth keys (short expiry);
  the node is tagged so ACLs constrain what it can reach.
- Lost stick → documented revoke path: expire/disable the Tailscale auth key and
  rotate the K3s agent token.
- Optional: LUKS-encrypt persistent/scratch storage (future flag).

## 7. Testing

Mirrors the existing `internal/ignition` test pattern (integration tests skip when
the external tool is absent):
- **Golden tests** (always run): `RenderJoinScript` output (agent/server, ephemeral
  + scratch variants, tailscale on/off, LAN-fallback); each renderer's payload
  (Butane YAML, cloud-init seed, baked config); `ventoy.json` generation; build
  dry-run plan.
- **Tool-gated integration** (skip if missing): `butane --strict` compile of the
  FCOS template; cloud-init schema validation if `cloud-init` present.
- **Reaper unit tests**: NotReady-ephemeral-past-TTL selection against fixture
  `kubectl get nodes -o json`.
- **Not automatable here:** real boot/join — documented field-test checklist in the
  PR and the guide.

## 8. Build order (within "all three now")
1. Foundation: `JoinSpec` + `RenderJoinScript` (+ golden tests); `coords`.
2. `render` interface + **FCOS** renderer (highest reuse) + `ventoy` assembler →
   first working stick artifact end-to-end at the generation layer.
3. **Ubuntu** renderer.
4. **Baked** renderer + config-partition layout.
5. `build` orchestrator + `cmd` wiring + config schema.
6. `prune` reaper + security hardening (agent-token plumbing, ephemeral labels).

## 9. Open questions / field validation
- Exact Tailscale-on-FCOS install method (static binary vs container) — script uses
  the static tarball; confirm on hardware.
- `coreos-installer iso ignition embed` vs Ventoy injection plugin for delivering
  Ignition — the assembler supports the injection-plugin path; embedding is a gated
  build step.
- Whether to ship the reaper as an in-cluster CronJob in addition to the CLI.
