# FCOS Tailscale + Join Recipe Redesign

> Status: design approved 2026-06-09. Fixes the observed failure where a USB-booted
> Fedora CoreOS node booted and applied Ignition but never joined the cluster.

## Problem

The shared join script (`internal/multinode/usb/joinspec.go`) installs Tailscale with
`curl tailscale.com/install.sh | sh`, which cannot work on **immutable Fedora CoreOS**
(no runtime `dnf`/package layering). Under `set -euo pipefail` that failure aborts the
whole join before k3s is ever installed. Live-observed on a NUC: FCOS booted, Ignition
applied, node never appeared in the cluster.

## Decision: Tailscale via static binary in Ignition

Download the **pinned** static tarball from `https://pkgs.tailscale.com/stable/tailscale_<ver>_amd64.tgz`
on first boot, install `tailscaled`/`tailscale` to `/usr/local/bin` (relabel with
`chcon -t bin_t` for SELinux Enforcing), and run `tailscaled` as a host systemd service in
**kernel mode** — so the `tailscale0` it creates is a real host interface that k3s
`--flannel-iface=tailscale0` + `--node-ip` can bind.

- **rpm-ostree layering — rejected:** package isn't live until a reboot (forces a two-boot flow).
- **podman container — rejected (heavier):** extra image pull, `TS_USERSPACE` default-mode trap, `podman exec` readiness probing.
- **static binary — chosen:** single-first-boot viable, no reboot, host-netns `tailscale0`, only a ~40MB download from an already-trusted upstream. Risk (SELinux 203/EXEC) mitigated by `chcon -t bin_t`.

Pin the version at **build** time: add `TailscaleVersion string` to `JoinSpec` (default e.g. `1.84.3`).

## FCOS Ignition (in `fcos.go` `fcosButane`, NOT the shared script)

Emitted only when Tailscale is enabled. Decoupled so a Tailscale failure can't block a LAN join.

1. **`/etc/devx/join.env`** (0600) — `TS_AUTHKEY`/`WIFI_PSK` move here, out of the 0755 script.
2. **`devx-tailscaled-install.service`** (oneshot, `ConditionPathExists=!/usr/local/bin/tailscaled`, `Before=tailscaled.service devx-join.service`) — curl tarball, install binaries, `chcon -t bin_t`.
3. **`tailscaled.service`** (kernel mode, our own unit — not the tarball's `/usr/sbin` one), `StateDirectory=tailscale` (`/var/lib/tailscale`, persistent).
4. **`devx-tailscale-up.service`** (oneshot, best-effort, `exit 0` even on failure) — `tailscale up`, then publish `/run/devx/tailnet-ip`. `After=time-sync.target`.
5. **`devx-join.service`** — `Wants=` (not `Requires=`) `devx-tailscale-up.service`; `Restart=on-failure`; `joined` marker written only after `k3s-agent`/`k3s` is actually active.
6. Drop the broken `devx-install.service` (referenced a script never written).

## Shared join script (`joinspec.go`)

- **Delete** the `install.sh | sh` + `tailscale up` block (lines ~242-249) — Tailscale now lives in FCOS units.
- Relax `set -euo pipefail` → `set -uo pipefail` (no blanket `-e`); critical steps check their own exit, best-effort steps use `|| true`.
- Tailnet branch consumes `/run/devx/tailnet-ip`, and waits for `tailscale0` to carry an IPv4 before committing.
- **Fix `--node-ip` bug:** when `FLANNEL_IFACE=tailscale0`, also emit `--node-ip=$NODE_IP` (and `--advertise-address` for server) — currently missing, so k3s advertises the LAN IP while flannel runs on tailscale0. Add a `node_ip_flag`.
- Publish `NODE_NAME` to `/run/devx/node-name` (so the up-unit's `--hostname` matches).
- Make the scratch bind fatal instead of `|| true`.

## Credential refresh

`build.go` already re-resolves `Token`/URLs via `ResolveCoordinates` per build. The baked
Tailscale auth key must be **ephemeral, ACL-tagged**. `TailscaleVersion` is resolved/defaulted at build time so first boot is offline-deterministic.

## Validation (before any hardware)

Boot **stock FCOS QEMU** with the rendered Ignition:
1. `coreos-installer download -s stable -p qemu -f qcow2.xz --decompress`.
2. `qemu-system-x86_64 ... -fw_cfg name=opt/com.coreos/config,file=devx-join.ign` (macOS: `-accel hvf`).
3. Confirm: install unit exits 0 + `ls -Z` shows `bin_t`; `tailscaled` active (not 203/EXEC); `tailscale0` gets a `100.x`; `devx-join` joins.
4. Cluster: `kubectl get nodes -o wide` shows the VM Ready with `INTERNAL-IP` = the `100.x` (proves the `--node-ip` fix) + `pool=usb`/`devx.io/ephemeral` labels.
5. Negative tests: block `pkgs.tailscale.com` → LAN join still works; blank authkey → LAN join still works; reboot → idempotent (`joined` marker), tailnet identity retained.

Only after the VM passes: generator Ventoy→GPT (coreos-installer) rework + re-flash + NUC test.
