# Self-Recovering Mac-Hosted Lima Nodes (node-vm-keeper)

> Status: design approved 2026-06-12. Makes Mac-hosted Lima k3s nodes recover on
> their own after a host reboot/sleep, instead of needing a manual rescue.

## Problem

A Mac host reboot (or bad sleep/wake) takes a Lima k3s node down and it does not
come back, because two things stack:

1. **`socket_vmnet` comes up wedged.** The plist devx writes (`prereqs.go`) has
   `RunAtLoad` but **no `KeepAlive`**, so a transient boot-time
   `vmnet_start_interface: VMNET_FAILURE` (interface not ready yet) kills it and
   launchd never retries → the VM's networking helper is dead.
2. **The Lima VM does not auto-start** on host boot, so nothing brings it back.

Observed live: `james-mbp` rebooted, `socket_vmnet` died on `VMNET_FAILURE`, the
`k8s-node` VM stayed `Stopped`, node `NotReady` until a manual restart.

## Design (two coordinated parts, installed during provisioning)

### Part 1 — `socket_vmnet` self-heals
Add `KeepAlive` (restart on non-zero exit) + a small `ThrottleInterval` to the
bridged-mode plist devx already writes in `prereqs.go`. Launchd then retries the
`VMNET_FAILURE` every few seconds until the interface is ready and the bridge
comes up. One-line-ish plist change; no new moving parts.

```xml
<key>KeepAlive</key>
<dict><key>SuccessfulExit</key><false/></dict>
<key>ThrottleInterval</key>
<integer>5</integer>
```

### Part 2 — `node-vm-keeper` LaunchAgent (user)
A per-host **LaunchAgent** (`io.devx.node-vm-keeper`) that runs a small idempotent
script: wait (bounded ~60s) for the `socket_vmnet` socket to exist (Part 1 heals
it), then `limactl start <vm>` **only if** the VM is not already `Running`.

- **LaunchAgent, not LaunchDaemon:** the VM uses the `vz` (Virtualization.framework)
  driver, which needs the user's GUI/Aqua session — a root daemon can't start it.
  Mirrors how devx already splits things (socket_vmnet = system daemon, VM = user).
- **Triggers:** `RunAtLoad` (recovers on reboot/login) **and** `StartInterval` 120s
  (a periodic watchdog that also recovers a VM that died mid-session after sleep/wake).
  Idempotent — it no-ops when the VM is healthy.
- **No root needed:** Part 1's `KeepAlive` owns socket_vmnet recovery, so the
  keeper only waits + starts the VM. No `sudo` in the agent.

**Install:** devx writes two files on the host (base64, like the existing
socket_vmnet plist): the keeper script (e.g. `~/.devx/node-vm-keeper.sh`, +x) and
`~/Library/LaunchAgents/io.devx.node-vm-keeper.plist`. LaunchAgents in
`~/Library/LaunchAgents` auto-load at the next GUI login, so no `launchctl
bootstrap` over SSH is required (which would fail without a GUI session). The
script is parameterized at install time with the detected `limactl` path, the VM
name, and the detected `socket_vmnet` socket path.

**Where in code:** a new `internal/multinode/lima/keeper.go` renders the script +
plist (pure, golden-tested); `lima.Manager` gains `InstallNodeKeeper(ctx)` called
at the end of `Provision` (after the VM is up), reusing `detectSocketPath`.

## Honest caveat

A LaunchAgent fires at **login**. For fully-headless boot recovery (no one at the
keyboard), the Mac needs **auto-login** enabled — documented, not flipped by devx
(it's security-sensitive). With auto-login on, recovery is fully hands-off.

## Testing

- Golden tests for the keeper script and the LaunchAgent plist (deterministic for
  given vmName/limactl/socket paths).
- Unit test that `InstallNodeKeeper` issues the expected write+chmod commands via a
  fake runner (mirrors the native-provider drop-in test).
- The `socket_vmnet` plist `KeepAlive` is covered by the existing prereqs plist
  assertions, extended to assert `KeepAlive`/`ThrottleInterval` are present.

## Out of scope

Enabling auto-login; recovering non-Mac (native/USB) nodes (those have their own
mechanisms already). FCOS/native nodes are unaffected.
