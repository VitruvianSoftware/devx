// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package usb generates the artifacts needed to boot a bare-metal laptop off a
// Ventoy USB stick and have it self-provision a K3s node that joins an existing
// devx-managed cluster.
//
// The package is organized around a single source of truth — the canonical join
// script produced by RenderJoinScript from a JoinSpec. Every OS renderer
// (FCOS/Ignition, Ubuntu/cloud-init, prebaked image) only re-packages that same
// script for its boot medium, so the join *logic* lives in exactly one place.
package usb

import (
	"fmt"
	"sort"
	"strings"
)

// EphemeralLabel marks nodes that joined via an ephemeral live-boot. The prune
// reaper uses it to distinguish disposable nodes from permanent ones.
const EphemeralLabel = "devx.io/ephemeral"

// Role is the K3s role a USB-booted node joins as.
type Role string

const (
	// RoleAgent joins as a worker. This is the default and the only safe choice
	// for ephemeral nodes: they must never become etcd voters.
	RoleAgent Role = "agent"
	// RoleServer joins as an additional control-plane (etcd) member. Only valid
	// for persistent installs.
	RoleServer Role = "server"
)

// Mode is the node lifecycle a boot entry provisions.
type Mode string

const (
	// ModeEphemeral runs the node from RAM; the internal disk is not installed
	// to. The node disappears on reboot/unplug.
	ModeEphemeral Mode = "ephemeral"
	// ModeInstall installs the OS to the internal disk for a permanent node.
	ModeInstall Mode = "install"
)

// ScratchStrategy selects how an ephemeral node obtains on-disk scratch space
// for containerd state and local-path volumes without destroying the operator's
// existing OS/data.
type ScratchStrategy string

const (
	// ScratchNone keeps everything in RAM (smallest footprint).
	ScratchNone ScratchStrategy = "none"
	// ScratchExisting mounts a pre-existing partition the operator designates by
	// LABEL=, UUID= or /dev/ path. Never formatted unless ForceFormat is set.
	ScratchExisting ScratchStrategy = "existing"
	// ScratchFreeSpace carves a new partition from unallocated space only. It
	// refuses to shrink or touch any existing partition.
	ScratchFreeSpace ScratchStrategy = "free-space"
)

// ScratchConfig describes ephemeral on-disk scratch storage.
type ScratchConfig struct {
	Strategy    ScratchStrategy
	Source      string // for ScratchExisting: "LABEL=...", "UUID=..." or "/dev/..."
	ForceFormat bool   // allow formatting an existing partition (destructive)
}

// JoinSpec is the fully-resolved description of how one boot entry should join
// the cluster. It is the input to RenderJoinScript and to every Renderer.
type JoinSpec struct {
	Role       Role
	Mode       Mode
	K3sVersion string // e.g. "v1.30.4+k3s1"; empty installs the k3s default
	Pool       string // node pool label value

	// NodeNamePrefix is combined at boot with the MAC address and hostname to
	// produce a name that is unique per laptop and per boot.
	NodeNamePrefix string

	Token            string // K3s join token (prefer a dedicated agent token)
	LANServerURL     string // e.g. "https://10.0.0.5:6443"; tried first if reachable
	TailnetServerURL string // e.g. "https://100.64.0.5:6443"; Tailscale fallback
	TailscaleAuthKey string // ephemeral, ACL-tagged; empty disables Tailscale

	Ephemeral   bool // add the EphemeralLabel and run as a disposable node
	Scratch     ScratchConfig
	ExtraLabels map[string]string
	ExtraArgs   []string // appended verbatim to the k3s install args

	WiFiSSID string // optional; bring up Wi-Fi before joining (for laptops w/o Ethernet)
	WiFiPSK  string
}

// nodeLabels returns the deterministic, sorted list of "k=v" node labels.
func (s JoinSpec) nodeLabels() []string {
	labels := map[string]string{}
	if s.Pool != "" {
		labels["pool"] = s.Pool
	}
	if s.Ephemeral {
		labels[EphemeralLabel] = "true"
	}
	for k, v := range s.ExtraLabels {
		labels[k] = v
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+labels[k])
	}
	return out
}

// Validate checks that the spec is internally consistent and safe.
func (s JoinSpec) Validate() error {
	switch s.Role {
	case RoleAgent, RoleServer:
	default:
		return fmt.Errorf("invalid role %q (want %q or %q)", s.Role, RoleAgent, RoleServer)
	}
	if s.Role == RoleServer && s.Ephemeral {
		return fmt.Errorf("ephemeral nodes must join as agents, not servers (etcd quorum safety)")
	}
	switch s.Mode {
	case ModeEphemeral, ModeInstall:
	default:
		return fmt.Errorf("invalid mode %q (want %q or %q)", s.Mode, ModeEphemeral, ModeInstall)
	}
	if s.Token == "" {
		return fmt.Errorf("token is required")
	}
	if s.LANServerURL == "" && s.TailnetServerURL == "" {
		return fmt.Errorf("at least one of LANServerURL or TailnetServerURL is required")
	}
	if s.TailnetServerURL != "" && s.TailscaleAuthKey == "" {
		return fmt.Errorf("TailnetServerURL set but no TailscaleAuthKey to reach it")
	}
	switch s.Scratch.Strategy {
	case "", ScratchNone, ScratchFreeSpace:
	case ScratchExisting:
		if s.Scratch.Source == "" {
			return fmt.Errorf("scratch strategy %q requires a source", ScratchExisting)
		}
	default:
		return fmt.Errorf("invalid scratch strategy %q", s.Scratch.Strategy)
	}
	return nil
}

// shellQuote single-quotes a value for safe embedding in a POSIX shell
// assignment, escaping any embedded single quotes.
func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}

// RenderJoinScript produces the canonical first-boot join script. The script is
// deterministic for a given spec (golden-tested) and OS-agnostic — renderers
// drop it onto their boot medium and arrange to run it once at first boot.
func RenderJoinScript(s JoinSpec) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}

	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	w("#!/usr/bin/env bash\n")
	w("# devx cluster node join — GENERATED, do not edit by hand.\n")
	w("# Wraps the canonical join recipe shared by all USB renderers.\n")
	w("set -euo pipefail\n\n")

	// --- Baked parameters --------------------------------------------------
	w("ROLE=%s\n", shellQuote(string(s.Role)))
	w("K3S_VERSION=%s\n", shellQuote(s.K3sVersion))
	w("TOKEN=%s\n", shellQuote(s.Token))
	w("LAN_SERVER_URL=%s\n", shellQuote(s.LANServerURL))
	w("TAILNET_SERVER_URL=%s\n", shellQuote(s.TailnetServerURL))
	w("TAILSCALE_AUTHKEY=%s\n", shellQuote(s.TailscaleAuthKey))
	w("NODE_NAME_PREFIX=%s\n", shellQuote(s.NodeNamePrefix))
	w("SCRATCH_STRATEGY=%s\n", shellQuote(string(s.Scratch.Strategy)))
	w("SCRATCH_SOURCE=%s\n", shellQuote(s.Scratch.Source))
	w("SCRATCH_FORCE_FORMAT=%s\n", shellQuote(boolStr(s.Scratch.ForceFormat)))
	w("NODE_LABELS=%s\n", shellQuote(strings.Join(s.nodeLabels(), ",")))
	w("WIFI_SSID=%s\n", shellQuote(s.WiFiSSID))
	w("WIFI_PSK=%s\n", shellQuote(s.WiFiPSK))
	w("\n")

	// --- Unique, stable-per-boot node name ---------------------------------
	// NOTE: these static shell blocks are written verbatim with WriteString,
	// never through the Fprintf-based w(), so literal '%' (shell parameter
	// expansions like ${x%%/*}) is not mangled as a printf verb.
	b.WriteString(`# Node name: prefix + first MAC (colons stripped) + short hostname.
mac="$(cat /sys/class/net/*/address 2>/dev/null | grep -v '^00:00:00:00:00:00$' | head -n1 | tr -d ':' || true)"
short_host="$(hostname -s 2>/dev/null || hostname)"
NODE_NAME="${NODE_NAME_PREFIX}-${mac:-nomac}-${short_host}"
echo "devx: node name ${NODE_NAME}"
`)
	w("\n")

	// --- Wi-Fi (optional; for laptops without Ethernet) --------------------
	b.WriteString(`if [ -n "$WIFI_SSID" ]; then
  echo "devx: connecting Wi-Fi ${WIFI_SSID}"
  if command -v nmcli >/dev/null 2>&1; then
    nmcli dev wifi connect "$WIFI_SSID" password "$WIFI_PSK" || true
  else
    echo "devx: nmcli not present; cannot bring up Wi-Fi" >&2
  fi
fi
`)
	w("\n")

	// --- Tailscale (optional) ----------------------------------------------
	b.WriteString(`if [ -n "$TAILSCALE_AUTHKEY" ]; then
  echo "devx: bringing up Tailscale"
  if ! command -v tailscale >/dev/null 2>&1; then
    curl -fsSL https://tailscale.com/install.sh | sh
  fi
  tailscale up --authkey="$TAILSCALE_AUTHKEY" --accept-routes --hostname="$NODE_NAME"
fi
`)
	w("\n")

	// --- Endpoint selection: prefer LAN if reachable, else tailnet ---------
	b.WriteString(`# devx_reachable HOST:PORT — true if a TCP connection succeeds within 3s.
devx_reachable() {
  local hostport="${1#*://}"; hostport="${hostport%%/*}"
  local host="${hostport%%:*}" port="${hostport##*:}"
  [ "$port" = "$host" ] && port=6443
  timeout 3 bash -c "exec 3<>/dev/tcp/${host}/${port}" 2>/dev/null
}

SERVER_URL=""
FLANNEL_IFACE=""
if [ -n "$LAN_SERVER_URL" ] && devx_reachable "$LAN_SERVER_URL"; then
  echo "devx: joining via LAN ${LAN_SERVER_URL}"
  SERVER_URL="$LAN_SERVER_URL"
elif [ -n "$TAILNET_SERVER_URL" ]; then
  echo "devx: joining via tailnet ${TAILNET_SERVER_URL}"
  SERVER_URL="$TAILNET_SERVER_URL"
  FLANNEL_IFACE="tailscale0"
elif [ -n "$LAN_SERVER_URL" ]; then
  echo "devx: LAN not reachable; trying ${LAN_SERVER_URL} anyway"
  SERVER_URL="$LAN_SERVER_URL"
else
  echo "devx: no server URL available" >&2
  exit 1
fi
`)
	w("\n")

	// --- Scratch storage (non-destructive) ---------------------------------
	b.WriteString(renderScratchBlock(s.Scratch))
	w("\n")

	// --- K3s install -------------------------------------------------------
	b.WriteString(renderK3sInstallBlock(s))

	return b.String(), nil
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// renderScratchBlock emits the shell that wires on-disk scratch storage for an
// ephemeral node. It is strictly non-destructive by default: ScratchExisting
// mounts but never formats (unless force), and ScratchFreeSpace only ever uses
// unallocated space.
func renderScratchBlock(c ScratchConfig) string {
	if c.Strategy == "" || c.Strategy == ScratchNone {
		return "echo \"devx: scratch=none (RAM only)\"\n"
	}

	var b strings.Builder
	b.WriteString(`SCRATCH_MNT=/var/lib/devx-scratch
mkdir -p "$SCRATCH_MNT"
`)
	switch c.Strategy {
	case ScratchExisting:
		b.WriteString(`echo "devx: using existing scratch ${SCRATCH_SOURCE}"
dev="$(blkid -t "${SCRATCH_SOURCE}" -o device 2>/dev/null || echo "$SCRATCH_SOURCE")"
if [ ! -b "$dev" ]; then echo "devx: scratch device $dev not found" >&2; exit 1; fi
if [ "$SCRATCH_FORCE_FORMAT" = "true" ]; then
  echo "devx: WARNING formatting $dev (--force-format)"; mkfs.ext4 -F "$dev"
fi
mount "$dev" "$SCRATCH_MNT"
`)
	case ScratchFreeSpace:
		b.WriteString(`echo "devx: carving scratch from unallocated space"
disk="$(lsblk -dpno NAME,TYPE | awk '$2=="disk"{print $1; exit}')"
if [ -z "$disk" ]; then echo "devx: no disk found for scratch" >&2; exit 1; fi
# Refuse unless there is unallocated space after the last partition.
free_mb="$(parted -sm "$disk" unit MB print free 2>/dev/null | awk -F: '/free;/{gsub(/MB/,"",$4); last=$4} END{print int(last)}')"
if [ -z "$free_mb" ] || [ "$free_mb" -lt 1024 ]; then
  echo "devx: <1GiB unallocated space on $disk; refusing to touch existing partitions" >&2
  exit 1
fi
parted -s "$disk" mkpart primary ext4 -- -"${free_mb}MB" 100%
newpart="$(lsblk -pno NAME "$disk" | tail -n1)"
mkfs.ext4 -F "$newpart"
mount "$newpart" "$SCRATCH_MNT"
`)
	}
	// Point k3s/containerd data at the scratch mount.
	b.WriteString(`mkdir -p "$SCRATCH_MNT/rancher" /var/lib/rancher
mount --bind "$SCRATCH_MNT/rancher" /var/lib/rancher || true
`)
	return b.String()
}

// renderK3sInstallBlock emits the get.k3s.io install invocation for the spec's
// role, mirroring internal/multinode/k3s so behavior matches SSH-provisioned
// nodes.
func renderK3sInstallBlock(s JoinSpec) string {
	var b strings.Builder
	b.WriteString(`flannel_flag=""
if [ -n "$FLANNEL_IFACE" ]; then flannel_flag="--flannel-iface=${FLANNEL_IFACE}"; fi
labels_flags=""
if [ -n "$NODE_LABELS" ]; then
  IFS=',' read -ra _labels <<< "$NODE_LABELS"
  for l in "${_labels[@]}"; do labels_flags="$labels_flags --node-label=$l"; done
fi
version_env=""
if [ -n "$K3S_VERSION" ]; then version_env="INSTALL_K3S_VERSION=$K3S_VERSION"; fi
`)
	extra := ""
	if len(s.ExtraArgs) > 0 {
		extra = " " + strings.Join(s.ExtraArgs, " ")
	}
	if s.Role == RoleServer {
		// Server join: --server + token, mirrors k3s.Manager.JoinServer.
		fmt.Fprintf(&b, `echo "devx: installing k3s server (joining HA control plane)"
curl -sfL https://get.k3s.io | env $version_env K3S_TOKEN="$TOKEN" INSTALL_K3S_EXEC="server" \
  sh -s - --server="$SERVER_URL" --node-name="$NODE_NAME" $flannel_flag $labels_flags%s
`, extra)
	} else {
		// Agent join: K3S_URL + token, mirrors k3s.Manager.JoinAgent.
		fmt.Fprintf(&b, `echo "devx: installing k3s agent (joining as worker)"
curl -sfL https://get.k3s.io | env $version_env K3S_TOKEN="$TOKEN" K3S_URL="$SERVER_URL" \
  sh -s - agent --node-name="$NODE_NAME" $flannel_flag $labels_flags%s
`, extra)
	}
	b.WriteString(`echo "devx: k3s install invoked; node ${NODE_NAME} joining ${SERVER_URL}"
`)
	return b.String()
}
