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

package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/k3s"
	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// sshSudo runs a command as root on the host and returns combined output.
type sshSudo func(ctx context.Context, command string) (string, error)

// NativeProvider runs K3s directly on a native Linux host (no Lima VM) over SSH.
// Handles immutable-OS deps (rpm-ostree), firewalld, Permissive SELinux, and
// tailnet networking. Supports both agent and server roles.
type NativeProvider struct {
	spec   ProviderSpec
	runner *remote.Runner
	run    sshSudo
	k3s    *k3s.Manager
}

// NewNativeProvider builds a NativeProvider for the node.
func NewNativeProvider(node config.NodeConfig, cfg *config.Config) *NativeProvider {
	r := util.NewRunner(node)
	p := &NativeProvider{
		spec: specFor(node, cfg), runner: r,
		run: func(ctx context.Context, cmd string) (string, error) { return r.RunSudo(ctx, cmd) },
	}
	// The k3s.Manager runs through p.run dynamically (honors a test-injected run).
	p.k3s = k3s.NewManagerWithExec(r, func(ctx context.Context, cmd string) (string, error) {
		return p.run(ctx, cmd)
	})
	return p
}

// detectPkgManager picks rpm-ostree (immutable) > dnf > apt.
func (p *NativeProvider) detectPkgManager(ctx context.Context) string {
	if out, _ := p.run(ctx, "command -v rpm-ostree"); strings.TrimSpace(out) != "" {
		return "rpm-ostree"
	}
	if out, _ := p.run(ctx, "command -v dnf"); strings.TrimSpace(out) != "" {
		return "dnf"
	}
	return "apt"
}

// depInstallCmd renders the package-install command for the detected manager.
// rpm-ostree uses --apply-live to avoid a reboot on immutable hosts.
func depInstallCmd(pm string, pkgs []string) string {
	list := strings.Join(pkgs, " ")
	switch pm {
	case "rpm-ostree":
		// --apply-live avoids a reboot; --idempotent skips an already-layered package.
		// There is deliberately no replacement override: rpm-ostree has no such flag
		// (--allow-replacement does not exist; --force-replacefiles errors on non-local
		// installs). The "packages would be changed: N" abort happens when a PRESENT
		// package would be UPDATED to the repo version on a stale base, which
		// --apply-live can't apply live — so callers gate on missingPkgs and only ever
		// pass genuinely-missing packages here.
		return "rpm-ostree install --apply-live --allow-inactive --idempotent " + list
	case "dnf":
		return "dnf install -y " + list
	default:
		return "apt-get update -qq && apt-get install -y -qq " + list
	}
}

// k3sDeps are required by K3s (conntrack) and devx bridge/port-forward (socat).
var k3sDeps = []string{"conntrack-tools", "socat"}

// k3s default cluster (pod) and service CIDRs. Flannel pod-bridge traffic uses
// these; on firewalld hosts they must be trusted or same-node pod dials are
// REJECTed ("no route to host"), which strands CSI volume attach (Longhorn).
const (
	k3sClusterCIDR = "10.42.0.0/16"
	k3sServiceCIDR = "10.43.0.0/16"
)

// missingPkgs returns the subset of pkgs NOT already installed (queried via rpm -q).
// rpm-ostree refuses to UPDATE a present package live on a stale base ("packages
// would be changed: N"), so the native reconcile must (re)install only genuinely-
// missing deps and leave present ones untouched.
func (p *NativeProvider) missingPkgs(ctx context.Context, pkgs []string) []string {
	var missing []string
	for _, pkg := range pkgs {
		out, _ := p.run(ctx, "rpm -q "+pkg+" >/dev/null 2>&1 && echo present || echo missing")
		if strings.Contains(out, "missing") {
			missing = append(missing, pkg)
		}
	}
	return missing
}

// ensureHostPrereqs converges a native host onto the full K3s/storage prereq
// set: packages (incl. iscsi for Longhorn), no-sleep, firewalld trust for the
// tailnet AND the k3s pod/service networks, and a running iscsid. Idempotent —
// called from both EnsureRuntime (first join) and Reconcile (heal existing
// nodes on `cluster apply`).
func (p *NativeProvider) ensureHostPrereqs(ctx context.Context) error {
	// 1. Install the K3s deps (conntrack, socat). On rpm-ostree/Silverblue, install
	// ONLY genuinely-missing packages: re-requesting a present package makes rpm-ostree
	// try to UPDATE it to the repo version, which a stale base can't apply live
	// ("packages would be changed: N") — and these deps ship in the base image, so it's
	// normally a no-op anyway. Best-effort + a loud warning rather than aborting the
	// whole reconcile; a genuinely-broken node surfaces at the readiness poll.
	// (iscsi-initiator-utils is intentionally NOT installed — Longhorn was removed;
	// storage is now NFS + local-path.)
	pm := p.detectPkgManager(ctx)
	deps := append([]string{}, k3sDeps...)
	if pm == "rpm-ostree" {
		deps = p.missingPkgs(ctx, deps)
	}
	if len(deps) > 0 {
		if out, err := p.run(ctx, depInstallCmd(pm, deps)); err != nil {
			slog.Warn("host dep install did not fully apply (continuing)", "host", p.spec.Host, "error", err, "output", out)
		}
	}
	// SELinux: the K3s install skips the k3s-selinux RPM (INSTALL_K3S_SKIP_SELINUX_RPM
	// in installScript), so on a STOCK enforcing host systemd cannot exec the freshly
	// downloaded k3s binary (labeled user_tmp_t) → "Permission denied" (203/EXEC). Make
	// SELinux permissive at runtime AND persist it, matching the skip-RPM intent. Both
	// are idempotent: setenforce errors when SELinux is already disabled/permissive, and
	// the sed only rewrites a still-enforcing config. Best-effort — a stuck SELinux
	// surfaces loudly at the post-join readiness poll, not silently. (#403)
	_, _ = p.run(ctx, "setenforce 0 2>/dev/null || true; "+
		"if [ -f /etc/selinux/config ]; then sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config; fi")
	// iptables backend: on Fedora's default nft backend, K3s's iptables-restore
	// mis-parses the host ruleset ("iptables-restore: COMMIT expected at line N") and
	// the agent dies at startup. Install the legacy backend and select it. Scoped to the
	// Fedora package managers (the `alternatives` tool and the package name are
	// RHEL-family); idempotent and a no-op where legacy is already active. (#403)
	if pm == "rpm-ostree" || pm == "dnf" {
		_, _ = p.run(ctx, depInstallCmd(pm, []string{"iptables-legacy"}))
		_, _ = p.run(ctx, "alternatives --set iptables /usr/sbin/iptables-legacy 2>/dev/null || true; "+
			"alternatives --set ip6tables /usr/sbin/ip6tables-legacy 2>/dev/null || true")
	}
	// tailscaled's unit declares a *required* EnvironmentFile=/etc/default/tailscaled.
	// On rpm-ostree/Silverblue the package ships it only as the OSTree default
	// /usr/etc/default/tailscaled, which the /etc merge can fail to materialize on
	// the deployment that first layers tailscale — leaving tailscaled (and thus a
	// k3s server pinned to --flannel-iface=tailscale0) dead after a reboot. Ensure
	// a persistent /etc copy. Self-gating on tailscaled's presence; best-effort.
	_, _ = p.run(ctx, "if command -v tailscaled >/dev/null 2>&1 && [ ! -f /etc/default/tailscaled ]; then "+
		"cp /usr/etc/default/tailscaled /etc/default/tailscaled 2>/dev/null || printf 'PORT=\"41641\"\\nFLAGS=\"\"\\n' > /etc/default/tailscaled; fi 2>/dev/null || true")
	// Keep the node awake: mask suspend/sleep so an idle GNOME/GDM login screen
	// (Workstation/Silverblue auto-suspends at the greeter) can't suspend the host
	// and silently drop it out of the cluster. Best-effort.
	_, _ = p.run(ctx, "systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target")
	// 2. Trust the tailnet interface AND the k3s pod/service networks in
	// firewalld. tailscale0 alone is not enough: pod-bridge (cni0) traffic is
	// otherwise REJECTed, breaking same-node pod dials AND pod egress to the
	// internet ("no route to host").
	//
	// Gate on firewalld being ENABLED, not on its momentary "running" state: on a
	// fresh join firewalld can be briefly stopped while the host converges, and
	// gating on live state silently skips the trust (it then only lands on a later
	// `cluster apply`), stranding egress-dependent pods until then. If enabled (or
	// already running), start it so --permanent + --reload take effect.
	// ("firewall-cmd --state" prints "not running" to stderr when stopped, hence
	// 2>/dev/null and the empty-stdout check.)
	fwEnabled, _ := p.run(ctx, "systemctl is-enabled firewalld 2>/dev/null || true")
	fwState, _ := p.run(ctx, "firewall-cmd --state 2>/dev/null || true")
	if strings.Contains(fwEnabled, "enabled") || strings.Contains(fwState, "running") {
		_, _ = p.run(ctx, "systemctl start firewalld 2>/dev/null || true")
		// Load-bearing: a failure here leaves pod/service traffic REJECTed, so
		// surface it instead of silently "succeeding" with broken networking.
		// --add-interface and --add-source must be in SEPARATE firewall-cmd calls:
		// newer firewall-cmd rejects mixing them in one invocation ("--add-source: not
		// allowed with argument --add-interface"). Add the interfaces, then the
		// pod/service source CIDRs, then reload. Both adds are idempotent.
		if _, err := p.run(ctx, fmt.Sprintf(
			"firewall-cmd --permanent --zone=trusted --add-interface=tailscale0 --add-interface=cni0 --add-interface=flannel.1 && "+
				"firewall-cmd --permanent --zone=trusted --add-source=%s --add-source=%s && firewall-cmd --reload",
			k3sClusterCIDR, k3sServiceCIDR)); err != nil {
			return fmt.Errorf("[%s] trusting k3s pod/service networks in firewalld: %w", p.spec.Host, err)
		}
	}
	return nil
}

func (p *NativeProvider) EnsureRuntime(ctx context.Context) (string, error) {
	if err := p.ensureHostPrereqs(ctx); err != nil {
		return "", err
	}
	// 3. Bring Tailscale up if a cluster auth key is configured (else assume it is up).
	if p.spec.Tailscale.Enabled && p.spec.Tailscale.AuthKey != "" {
		_, _ = p.run(ctx, fmt.Sprintf(
			"command -v tailscale >/dev/null 2>&1 && (tailscale status >/dev/null 2>&1 || tailscale up --authkey=%s --accept-routes)",
			p.spec.Tailscale.AuthKey))
		// Enforce --accept-routes idempotently. The `|| tailscale up` above is a
		// no-op when the node is already up (joined earlier without accept-routes),
		// leaving RouteAll=false — which broke the Fedora/nuc9 nodes under Cilium
		// native routing over tailscale. `tailscale set` re-asserts it regardless.
		// Load-bearing: surface failure (mirrors k3s.Manager.InstallTailscale).
		if _, err := p.run(ctx, "command -v tailscale >/dev/null 2>&1 && tailscale set --accept-routes=true"); err != nil {
			return "", fmt.Errorf("[%s] enforcing tailscale accept-routes: %w", p.spec.Host, err)
		}
	}
	// 4. Node IP: explicit override, else the tailscale IP.
	if p.spec.NodeIP != "" {
		return p.spec.NodeIP, nil
	}
	out, err := p.run(ctx, "tailscale ip -4")
	if err != nil {
		return "", fmt.Errorf("[%s] reading tailscale ip: %w", p.spec.Host, err)
	}
	return strings.TrimSpace(out), nil
}

// installScript renders the get.k3s.io install for the given role. Same recipe as
// the Lima path, with SELinux RPM skipped (Permissive) and run directly on the host.
func (p *NativeProvider) installScript(role string, o JoinOpts) string {
	verEnv := ""
	if o.K3sVersion != "" {
		verEnv = fmt.Sprintf("INSTALL_K3S_VERSION=%q ", o.K3sVersion)
	}
	flannel := ""
	if o.UseTailscale {
		flannel = " --flannel-iface=tailscale0"
	}
	common := fmt.Sprintf("%sINSTALL_K3S_SKIP_SELINUX_RPM=true K3S_TOKEN=%q", verEnv, o.Token)
	// dataDir relocates k3s's data-dir (etcd/containerd/local-path storage) AND the
	// kubelet root-dir (the node's reported ephemeral-storage) onto spec.DataDir. Used
	// for native hosts whose / is too small for the cluster while the bulk of the disk
	// sits on a separate volume (e.g. a repurposed Workstation with a large /home).
	// Empty = k3s default (/var/lib/rancher/k3s + /var/lib/kubelet).
	dataDir := ""
	if p.spec.DataDir != "" {
		dataDir = fmt.Sprintf(" --data-dir=%s --kubelet-arg=root-dir=%s/kubelet", p.spec.DataDir, p.spec.DataDir)
	}
	if role == "server" {
		var sans string
		for _, s := range o.TLSSANs {
			sans += fmt.Sprintf(" --tls-san=%s", s)
		}
		lb := ""
		if o.DisableServiceLB {
			lb = " --disable servicelb"
		}
		cilium := ""
		if o.UseCilium {
			// k3s embeds Flannel + kube-proxy + the netpol controller; Cilium replaces all three.
			cilium = " --flannel-backend=none --disable-kube-proxy --disable-network-policy"
		}
		return fmt.Sprintf(
			`curl -sfL https://get.k3s.io | %s INSTALL_K3S_EXEC="server" sh -s - --server=%s --node-name=%s --node-ip=%s --advertise-address=%s%s%s%s%s%s --node-label=pool=%s`,
			common, o.ServerURL, p.spec.Host, o.NodeIP, o.NodeIP, sans, flannel, lb, cilium, dataDir, o.Pool)
	}
	return fmt.Sprintf(
		`curl -sfL https://get.k3s.io | %s K3S_URL=%q sh -s - agent --node-name=%s --node-ip=%s%s%s --node-label=pool=%s`,
		common, o.ServerURL, p.spec.Host, o.NodeIP, flannel, dataDir, o.Pool)
}

// ensureKubeletSymlink points the standard /var/lib/kubelet at the relocated
// kubelet root (<dataDir>/kubelet) when a node uses a custom dataDir.
// --kubelet-arg=root-dir moves the kubelet, but CSI drivers and other DaemonSets
// hardcode hostPaths under /var/lib/kubelet (pods, plugins, plugins_registry); without
// this symlink their mounts fail with "hostPath type check failed". No-op without a
// dataDir; idempotent (re-points an existing symlink, replaces a stray real dir).
func (p *NativeProvider) ensureKubeletSymlink(ctx context.Context) error {
	if p.spec.DataDir == "" {
		return nil
	}
	root := strings.TrimRight(p.spec.DataDir, "/") + "/kubelet"
	cmd := fmt.Sprintf("mkdir -p %s && { [ -L /var/lib/kubelet ] || rm -rf /var/lib/kubelet; } && ln -sfn %s /var/lib/kubelet", root, root)
	_, err := p.run(ctx, cmd)
	return err
}

func (p *NativeProvider) InstallAgent(ctx context.Context, o JoinOpts) error {
	if inst, _ := p.IsInstalled(ctx); !inst {
		if err := p.ensureKubeletSymlink(ctx); err != nil {
			return err
		}
		if _, err := p.run(ctx, p.installScript("agent", o)); err != nil {
			return err
		}
	}
	return p.ensureTailscaleOrdering(ctx, "agent", o)
}

func (p *NativeProvider) InstallServer(ctx context.Context, o JoinOpts) error {
	if inst, _ := p.IsInstalled(ctx); !inst {
		if err := p.ensureKubeletSymlink(ctx); err != nil {
			return err
		}
		if _, err := p.run(ctx, p.installScript("server", o)); err != nil {
			return err
		}
	}
	return p.ensureTailscaleOrdering(ctx, "server", o)
}

// tailscaleOrderingConf returns the systemd drop-in directory and contents that
// order the K3s unit after tailscaled and wait (bounded to 60s) for tailscale0 to
// carry an IPv4 before K3s launches.
func tailscaleOrderingConf(role string) (dir, contents string) {
	unit := "k3s-agent.service"
	if role == "server" {
		unit = "k3s.service"
	}
	dir = "/etc/systemd/system/" + unit + ".d"
	contents = "[Unit]\n" +
		"After=tailscaled.service\n" +
		"Wants=tailscaled.service\n\n" +
		"[Service]\n" +
		"ExecStartPre=/usr/bin/bash -c 'for i in $(seq 1 60); do ip -4 addr show tailscale0 2>/dev/null | grep -q \"inet \" && break; sleep 1; done'\n"
	return dir, contents
}

// ensureTailscaleOrdering installs a systemd drop-in so the K3s unit starts after
// tailscaled and waits for tailscale0's IPv4. The install pins
// --flannel-iface=tailscale0 and --node-ip to the tailnet address, so on a reboot
// that brings K3s up before the tailnet is ready the node would fail and thrash
// until Restart recovers it; ordering + a short wait make the rejoin prompt and
// clean. Idempotent, and a no-op when Tailscale is not in use.
func (p *NativeProvider) ensureTailscaleOrdering(ctx context.Context, role string, o JoinOpts) error {
	if !o.UseTailscale {
		return nil
	}
	dir, conf := tailscaleOrderingConf(role)
	b64 := base64.StdEncoding.EncodeToString([]byte(conf))
	cmd := fmt.Sprintf("mkdir -p %s && echo %s | base64 -d > %s/10-devx-tailscale.conf && systemctl daemon-reload", dir, b64, dir)
	_, err := p.run(ctx, cmd)
	return err
}

// The remaining lifecycle ops are identical to Lima — delegate to the exec-backed
// k3s.Manager, which runs the same commands directly on the host.
func (p *NativeProvider) GetToken(ctx context.Context) (string, error) { return p.k3s.GetToken(ctx) }
func (p *NativeProvider) IsInstalled(ctx context.Context) (bool, error) {
	return p.k3s.IsInstalled(ctx)
}
func (p *NativeProvider) WaitForReady(ctx context.Context, d time.Duration) error {
	return p.k3s.WaitForReady(ctx, d)
}
func (p *NativeProvider) NodeStatus(ctx context.Context) (string, error) {
	return p.k3s.GetNodeStatus(ctx)
}
func (p *NativeProvider) Kubeconfig(ctx context.Context, ip string) (string, error) {
	return p.k3s.GetKubeconfig(ctx, ip)
}
func (p *NativeProvider) Drain(ctx context.Context, n string) error { return p.k3s.DrainNode(ctx, n) }
func (p *NativeProvider) DeleteNode(ctx context.Context, n string) error {
	return p.k3s.DeleteNode(ctx, n)
}
func (p *NativeProvider) Uninstall(ctx context.Context, role string) error {
	return p.k3s.Uninstall(ctx, role)
}

// Destroy uninstalls K3s; there is no VM to remove on a native host.
func (p *NativeProvider) Destroy(ctx context.Context) error {
	return p.k3s.Uninstall(ctx, p.spec.Role)
}

func (p *NativeProvider) Reconcile(ctx context.Context) error {
	return p.ensureHostPrereqs(ctx)
}
