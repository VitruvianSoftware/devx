# devx cluster USB — build manifest

Boot menu entries:

- Join cluster (ephemeral) — Fedora CoreOS  (fcos/ephemeral, ignition)
- Join cluster (ephemeral) — Ubuntu (Wi-Fi friendly)  (ubuntu/ephemeral, cloud-init)

Required base images (copy these onto the Ventoy stick root):

- fedora-coreos-live.x86_64.iso
- ubuntu-24.04-live-server-amd64.iso

The provisioning payloads (Ignition/cloud-init/config) and ventoy/ventoy.json
are staged alongside this manifest. To skip the manual assembly, re-run with
--assemble (build a flashable .img in a Lima VM) or --device /dev/diskN (build
and flash a removable stick directly). See the design spec for the field
checklist (Secure Boot off, Ethernet recommended for FCOS, x86_64 only).
