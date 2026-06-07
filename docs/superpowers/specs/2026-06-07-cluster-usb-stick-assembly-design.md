# Design: One-command bootable USB assembly + flash (`devx cluster usb build --device`)

- **Status:** Approved (design), implementation pending
- **Date:** 2026-06-07
- **Author:** James H. Nguyen (with Claude)
- **Topic slug:** `cluster-usb-stick-assembly`
- **Builds on:** [`2026-06-06-cluster-usb-boot-nodes-design.md`](2026-06-06-cluster-usb-boot-nodes-design.md)

## 1. Goal

The first cut of `devx cluster usb build` generates the provisioning *payloads*
(Ignition, cloud-init seeds, join script, `ventoy.json`) into a folder, but a
human still has to (a) download the OS ISOs, (b) embed the Ignition into the FCOS
ISO, (c) install Ventoy onto the USB, and (d) copy everything across. This design
**bakes those manual assembly steps into the command** so one invocation produces
a finished, bootable, self-joining Ventoy stick.

It also covers setting up the operator's **physical SanDisk** (`/dev/disk4`, a
123 GB USB) as the real-hardware validation of the build+flash path.

## 2. Decisions (from brainstorming)

- **USB layout:** a small **16 GB Ventoy boot partition** (holds the ISOs +
  payloads) + the **remaining ~107 GB as a normal exFAT storage partition**.
  Configurable via `--boot-size` (default `16G`).
- **Assembly approach:** Ventoy install + Ignition embed are Linux-only and
  cannot run on macOS, so devx **builds a complete disk image inside a Lima Linux
  VM**, then **flashes it to the device from macOS** (`diskutil` + `dd`). This is
  reliable on Apple Silicon (no USB passthrough, which vz does not support).
- **Stick contents:** **ephemeral entries only** (FCOS + Ubuntu, `scratch=none`).
  The install-to-disk path is **deferred** (see §8) so booting this stick can
  never format a host laptop's internal disk.
- **Scope of "bake in":** download ISOs → embed Ignition → install Ventoy →
  partition (boot + storage) → copy payloads → flash. All from one command.

## 3. Host-disk safety (the load-bearing safety property)

Two independent safety concerns, kept separate:

1. **Booted-laptop safety.** Ephemeral entries run entirely in RAM. The FCOS
   Ignition writes only `files:` (no `disks:`/`filesystems:`/`wipe`), and the join
   script with `scratch=none` performs zero disk operations. Verified against the
   generated artifacts. Shipping **ephemeral-only** means a wrong menu pick cannot
   wipe a borrowed laptop. Install entries (which *do* wipe the host disk by
   design) are out of scope here.
2. **Flash-target safety.** `--device` overwrites the whole target, so before any
   write devx **refuses non-removable targets** (`diskutil info` must show
   `Internal: No`, ejectable/removable, USB protocol), hard-refuses `disk0`/the
   system disk, prints the target identity, and requires explicit confirmation
   unless `--yes`. `--dry-run` writes nothing.

## 4. Command surface

Extend the existing `devx cluster usb build`:

- **`devx cluster usb build`** (no `--device`) → assembles the flashable
  `.img` in Lima and writes it to `--image-out` (default `devx-usb.img`). Safe;
  no device touched.
- **`devx cluster usb build --device /dev/diskN`** → also flashes the image to
  the device and ejects it. The one-command "ready stick."

Flags (additive to today's `--renderer/--mode/--dry-run/--json`):

| Flag | Default | Meaning |
|---|---|---|
| `--device` | (none) | flash target; triggers the destructive write path |
| `--boot-size` | `16G` | Ventoy boot partition size; remainder → exFAT storage |
| `--total-size` | device size, else `32G` | total image size; with `--device` defaults to the device's size, otherwise sizes the image-only output |
| `--image-out` | `devx-usb.img` | where to write the assembled image |
| `--isos` | `fcos,ubuntu` | which OS images to include (ephemeral) |
| `--yes` | `false` | skip the destructive-write confirmation |
| `--builder-vm` | `devx-usb-builder` | name of the Lima builder VM |
| `--keep-builder` | `false` | don't delete the builder VM afterward (debug) |

For this stick, the assembled entries are **ephemeral-only** (the command forces
`--mode ephemeral` for the boot menu; install entries are not emitted).

## 5. Architecture & components

New code lives in `internal/multinode/usb` (flat package, consistent with today):

- `assemble.go` — orchestrates the Lima build:
  - `EnsureBuilderVM(ctx, name)` — create/start an ephemeral Lima VM provisioned
    with the build tooling (idempotent).
  - `BuildImage(ctx, opts) (imagePath string, err error)` — stage payloads, copy
    them + the assembly script into the VM, run it, copy the resulting `.img` out.
- `assemble_script.go` — generates the **in-VM assembly shell script** (pure text,
  golden-tested). The script, run inside the Linux VM, does:
  1. download the chosen ISOs (FCOS x86_64 live, Ubuntu 24.04 live-server amd64),
     cached under a persistent dir;
  2. `coreos-installer iso ignition embed` the Ignition into a copy of the FCOS ISO;
  3. `truncate` a sparse image sized to the target (device size, or `--total-size`);
  4. `losetup` it, run Ventoy's `Ventoy2Disk.sh -I -r <reserveMB>` (force,
     non-interactive, `reserveMB = totalMB - bootMB`) on the loop device so the
     Ventoy boot partition is `--boot-size` and the rest is reserved;
  5. `parted` + `mkfs.exfat` the reserved space → storage partition;
  6. mount the Ventoy data partition, copy in the ISOs + `ventoy.json` + the
     Ubuntu cloud-init seed (NoCloud);
  7. detach the loop; the sparse image now holds the full layout.
- `flash.go` (macOS) — `ValidateRemovableDevice(device)` (via `diskutil info`),
  `Flash(ctx, imagePath, device)` = `diskutil unmountDisk` → `dd` →
  `diskutil eject`, with the §3 confirmation gate.
- `builder_provision.go` — the Lima VM provision script: install `wget`, the
  Ventoy Linux release tarball, `coreos-installer` (static binary), `exfatprogs`,
  `parted`, `dosfstools`.
- `cmd/cluster_mgmt_usb.go` — extend `build` with the §4 flags; wire to
  `assemble` + `flash`.

The existing renderers are reused unchanged to produce the payloads that the
assembly script consumes (Ignition Butane→`.ign`, cloud-init seed, `ventoy.json`).

## 6. Data flow

```
devx cluster usb build [--device /dev/disk4]
  1. resolve coords → JoinSpec (ephemeral, scratch=none) → render payloads to a staging dir
  2. EnsureBuilderVM: ephemeral Lima VM with ventoy + coreos-installer + exfatprogs + parted
  3. copy staging payloads + generated assembly script into the VM
  4. run assembly script in VM  →  /tmp/devx-usb.img  (full layout: 16G Ventoy boot + ~107G exFAT)
  5. copy the .img out to the host (sparse-aware)
  6. (no --device) write to --image-out and stop
     (--device) ValidateRemovableDevice → confirm → diskutil unmountDisk → dd → eject
  7. result: bootable stick — [16G boot: FCOS+Ubuntu ISOs, ignition embedded, ventoy.json] + [~107G exFAT]
```

## 7. Testing

- **Unit (always run):** golden test for the generated in-VM assembly script and
  the builder provision script; `flash.go` device-validation (table-driven over
  fixture `diskutil info` output — internal disk → refuse, removable USB → allow);
  flag parsing / build-plan; `--dry-run` plan output.
- **Integration (gated on Lima/tooling, opt-in):** run the real assembly in a
  Lima VM and **loopback-inspect the produced image** — assert the Ventoy boot
  partition + exFAT storage partition exist, both ISOs are present, and the FCOS
  ISO has the Ignition embedded (`coreos-installer iso ignition show`).
- **Real hardware:** flashing the operator's SanDisk and re-reading its partition
  table / mounting the storage partition is the build+flash integration test.
- **Not automatable here:** the x86 boot + join — field test on a real laptop
  (this is an arm64 Mac; the feature targets x86_64).

## 8. Out of scope / follow-ups

- **Install-to-disk entries** (FCOS `coreos-installer install`, Ubuntu autoinstall)
  — deferred; needs the FCOS installer finished + a destructive-confirmation UX.
- **`baked` renderer on the stick** — needs its prebuilt-image pipeline first.
- **x86 boot proof** — operator field test.
- **Wi-Fi-only laptops** — SSID/PSK baking already exists in the join script;
  validating FCOS-live Wi-Fi firmware is a field concern.

## 9. Open implementation details (pinned in the plan)

- **Flash mechanics:** correctness is unambiguous (build the full layout in the VM,
  flash the whole image). The *optimization* — avoiding a full 123 GB write by
  using a sparse-aware copy — depends on macOS `dd` capabilities and is settled in
  implementation; the acceptable fallback is a full write (slower, still correct).
- **FCOS ISO URL resolution:** use the Fedora CoreOS stable-stream metadata JSON to
  resolve the current live ISO URL (don't hardcode a version).
- **coreos-installer in the VM:** install via the official static binary release.
