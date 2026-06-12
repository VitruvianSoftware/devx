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
	node   config.NodeConfig
	cfg    *config.Config
	runner *remote.Runner
	run    sshSudo
	k3s    *k3s.Manager
}

// NewNativeProvider builds a NativeProvider for the node.
func NewNativeProvider(node config.NodeConfig, cfg *config.Config) *NativeProvider {
	r := util.NewRunner(node)
	p := &NativeProvider{
		node: node, cfg: cfg, runner: r,
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

// iscsiPkg returns the open-iscsi package name for the package manager.
// Longhorn requires iscsiadm on every node; the Lima path already installs it
// (lima.NodePackages), the native path must too.
func iscsiPkg(pm string) string {
	if pm == "apt" {
		return "open-iscsi"
	}
	return "iscsi-initiator-utils"
}

// ensureHostPrereqs converges a native host onto the full K3s/storage prereq
// set: packages (incl. iscsi for Longhorn), no-sleep, firewalld trust for the
// tailnet AND the k3s pod/service networks, and a running iscsid. Idempotent —
// called from both EnsureRuntime (first join) and Reconcile (heal existing
// nodes on `cluster apply`).
func (p *NativeProvider) ensureHostPrereqs(ctx context.Context) error {
	// 1. Install K3s deps via the detected package manager.
	pm := p.detectPkgManager(ctx)
	if _, err := p.run(ctx, depInstallCmd(pm, append(append([]string{}, k3sDeps...), iscsiPkg(pm)))); err != nil {
		return fmt.Errorf("[%s] installing deps: %w", p.node.Host, err)
	}
	// Longhorn needs the iscsid daemon and the iscsi_tcp module. Best-effort:
	// nodes without Longhorn workloads lose nothing if this fails.
	_, _ = p.run(ctx, "systemctl enable --now iscsid 2>/dev/null || true; modprobe iscsi_tcp 2>/dev/null || true")
	// Keep the node awake: mask suspend/sleep so an idle GNOME/GDM login screen
	// (Workstation/Silverblue auto-suspends at the greeter) can't suspend the host
	// and silently drop it out of the cluster. Best-effort.
	_, _ = p.run(ctx, "systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target")
	// 2. Trust the tailnet interface AND the k3s pod/service networks in
	// firewalld (if running). tailscale0 alone is not enough: pod-bridge (cni0)
	// traffic is otherwise REJECTed, breaking same-node pod dials.
	if out, _ := p.run(ctx, "firewall-cmd --state 2>/dev/null || true"); strings.Contains(out, "running") {
		_, _ = p.run(ctx, fmt.Sprintf(
			"firewall-cmd --permanent --zone=trusted --add-interface=tailscale0 --add-interface=cni0 --add-interface=flannel.1 --add-source=%s --add-source=%s && firewall-cmd --reload",
			k3sClusterCIDR, k3sServiceCIDR))
	}
	return nil
}

func (p *NativeProvider) EnsureRuntime(ctx context.Context) (string, error) {
	if err := p.ensureHostPrereqs(ctx); err != nil {
		return "", err
	}
	// 3. Bring Tailscale up if a cluster auth key is configured (else assume it is up).
	if p.cfg.Cluster.Tailscale.Enabled && p.cfg.Cluster.Tailscale.AuthKey != "" {
		_, _ = p.run(ctx, fmt.Sprintf(
			"command -v tailscale >/dev/null 2>&1 && (tailscale status >/dev/null 2>&1 || tailscale up --authkey=%s --accept-routes)",
			p.cfg.Cluster.Tailscale.AuthKey))
	}
	// 4. Node IP: explicit override, else the tailscale IP.
	if p.node.NodeIP != "" {
		return p.node.NodeIP, nil
	}
	out, err := p.run(ctx, "tailscale ip -4")
	if err != nil {
		return "", fmt.Errorf("[%s] reading tailscale ip: %w", p.node.Host, err)
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
	if role == "server" {
		var sans string
		for _, s := range o.TLSSANs {
			sans += fmt.Sprintf(" --tls-san=%s", s)
		}
		lb := ""
		if o.DisableServiceLB {
			lb = " --disable servicelb"
		}
		return fmt.Sprintf(
			`curl -sfL https://get.k3s.io | %s INSTALL_K3S_EXEC="server" sh -s - --server=%s --node-name=%s --node-ip=%s --advertise-address=%s%s%s%s --node-label=pool=%s`,
			common, o.ServerURL, p.node.Host, o.NodeIP, o.NodeIP, sans, flannel, lb, o.Pool)
	}
	return fmt.Sprintf(
		`curl -sfL https://get.k3s.io | %s K3S_URL=%q sh -s - agent --node-name=%s --node-ip=%s%s --node-label=pool=%s`,
		common, o.ServerURL, p.node.Host, o.NodeIP, flannel, o.Pool)
}

func (p *NativeProvider) InstallAgent(ctx context.Context, o JoinOpts) error {
	if inst, _ := p.IsInstalled(ctx); !inst {
		if _, err := p.run(ctx, p.installScript("agent", o)); err != nil {
			return err
		}
	}
	return p.ensureTailscaleOrdering(ctx, "agent", o)
}

func (p *NativeProvider) InstallServer(ctx context.Context, o JoinOpts) error {
	if inst, _ := p.IsInstalled(ctx); !inst {
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
	return p.k3s.Uninstall(ctx, p.node.Role)
}

func (p *NativeProvider) Reconcile(ctx context.Context) error {
	return p.ensureHostPrereqs(ctx)
}
