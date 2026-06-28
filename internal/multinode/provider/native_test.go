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
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

// nativeWithRunner builds a NativeProvider whose command runner is replaced with
// a fake that records calls and answers by substring match.
func nativeWithRunner(replies map[string]string) (*NativeProvider, *[]string) {
	var calls []string
	p := NewNativeProvider(config.NodeConfig{Host: "fedora", Kind: "native", Role: "agent", Pool: "linux"}, &config.Config{})
	p.run = func(ctx context.Context, cmd string) (string, error) {
		calls = append(calls, cmd)
		for k, v := range replies {
			if strings.Contains(cmd, k) {
				return v, nil
			}
		}
		return "", nil
	}
	return p, &calls
}

func TestNative_EnsureRuntime_Silverblue(t *testing.T) {
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"firewall-cmd --state":  "running",
		"tailscale ip -4":       "100.97.82.15",
	})
	ip, err := p.EnsureRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ip != "100.97.82.15" {
		t.Errorf("node ip = %q, want the tailscale ip", ip)
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"rpm-ostree install",
		"--apply-live",
		"conntrack-tools socat",
		"systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target",
		"firewall-cmd --permanent --zone=trusted --add-interface=tailscale0",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("EnsureRuntime missing %q in:\n%s", want, joined)
		}
	}
}

func TestNative_EnsureRuntime_EnforcesAcceptRoutes(t *testing.T) {
	// Regression: `tailscale status >/dev/null 2>&1 || tailscale up ... --accept-routes`
	// never sets accept-routes on a node that is ALREADY up (joined earlier without
	// it), leaving RouteAll=false and breaking Cilium native routing over tailscale
	// (the Fedora/nuc9 nodes). EnsureRuntime must ALWAYS enforce it after `up`.
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"tailscale ip -4":       "100.97.82.15",
	})
	// A cluster auth key must be configured for the tailscale-up step to run.
	p.spec.Tailscale.Enabled = true
	p.spec.Tailscale.AuthKey = "tskey-abc"
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	if !strings.Contains(joined, "tailscale set --accept-routes=true") {
		t.Errorf("EnsureRuntime must enforce `tailscale set --accept-routes=true`; got:\n%s", joined)
	}
}

func TestNative_EnsureRuntime_NodeIPOverride(t *testing.T) {
	p, _ := nativeWithRunner(nil)
	p.spec.NodeIP = "10.0.0.9"
	ip, err := p.EnsureRuntime(context.Background())
	if err != nil || ip != "10.0.0.9" {
		t.Errorf("nodeIP override should win: %q,%v", ip, err)
	}
}

func TestNative_InstallAgent_Recipe(t *testing.T) {
	p, calls := nativeWithRunner(nil)
	err := p.InstallAgent(context.Background(), JoinOpts{
		NodeIP: "100.97.82.15", ServerURL: "https://100.64.0.5:6443", Token: "K10tok",
		Pool: "linux", K3sVersion: "v1.30.4+k3s1", UseTailscale: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(*calls, "\n")
	for _, want := range []string{
		"get.k3s.io",
		`K3S_URL="https://100.64.0.5:6443"`,
		`K3S_TOKEN="K10tok"`,
		"sh -s - agent",
		"--flannel-iface=tailscale0",
		"--node-ip=100.97.82.15",
		"--node-label=pool=linux",
		"INSTALL_K3S_SKIP_SELINUX_RPM=true",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("agent install missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "--server=") {
		t.Error("agent must not use --server")
	}
}

func TestNative_InstallServer_Recipe(t *testing.T) {
	p, calls := nativeWithRunner(nil)
	err := p.InstallServer(context.Background(), JoinOpts{
		NodeIP: "100.97.82.15", ServerURL: "https://100.64.0.5:6443", Token: "K10tok",
		Pool: "linux", TLSSANs: []string{"100.97.82.15"}, UseTailscale: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(*calls, "\n")
	if !strings.Contains(got, "--server=https://100.64.0.5:6443") || !strings.Contains(got, `INSTALL_K3S_EXEC="server"`) {
		t.Errorf("server install must use --server + server exec:\n%s", got)
	}
}

func TestNative_DepInstall_PackageManagers(t *testing.T) {
	cases := map[string]string{
		"rpm-ostree": "rpm-ostree install --apply-live",
		"dnf":        "dnf install -y",
		"apt":        "apt-get install -y",
	}
	for pm, want := range cases {
		if got := depInstallCmd(pm, []string{"conntrack-tools", "socat"}); !strings.Contains(got, want) {
			t.Errorf("depInstallCmd(%s) = %q, want substring %q", pm, got, want)
		}
	}
}

func TestNative_DepInstall_RpmOstreeAllowsReplacement(t *testing.T) {
	// On Silverblue, a layered prereq (e.g. iscsi-initiator-utils) can replace a
	// base-image package; without --allow-replacement, `rpm-ostree install
	// --apply-live` aborts with "packages would be changed: N, allow replacement
	// to override" and the prereq never lands — so it vanishes after a reboot and
	// longhorn-manager crashloops (no iscsiadm).
	got := depInstallCmd("rpm-ostree", []string{"iscsi-initiator-utils"})
	if !strings.Contains(got, "--allow-replacement") {
		t.Errorf("rpm-ostree install must pass --allow-replacement, got %q", got)
	}
}

func TestNative_EnsureRuntime_PersistsISCSIModuleAndTailscaledEnv(t *testing.T) {
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"tailscale ip -4":       "100.97.82.15",
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")

	// iscsi_tcp must be persisted (modules-load.d) so it auto-loads after a
	// reboot — a live `modprobe` alone is lost, leaving Longhorn unable to attach.
	if !strings.Contains(joined, "modules-load.d") || !strings.Contains(joined, "iscsi_tcp") {
		t.Errorf("EnsureRuntime must persist iscsi_tcp via modules-load.d, got:\n%s", joined)
	}

	// tailscaled's unit has a *required* EnvironmentFile=/etc/default/tailscaled;
	// the rpm-ostree /etc merge can drop it on the deploy that first layers
	// tailscale, killing tailscaled (and k3s --flannel-iface=tailscale0) after a
	// reboot. Provisioning must ensure a persistent /etc copy.
	if !strings.Contains(joined, "/etc/default/tailscaled") {
		t.Errorf("EnsureRuntime must ensure /etc/default/tailscaled exists, got:\n%s", joined)
	}
}

func TestNative_TailscaleOrderingConf(t *testing.T) {
	dir, conf := tailscaleOrderingConf("agent")
	if dir != "/etc/systemd/system/k3s-agent.service.d" {
		t.Errorf("agent drop-in dir = %q", dir)
	}
	if d, _ := tailscaleOrderingConf("server"); d != "/etc/systemd/system/k3s.service.d" {
		t.Errorf("server drop-in dir = %q", d)
	}
	for _, want := range []string{
		"After=tailscaled.service",
		"Wants=tailscaled.service",
		"ExecStartPre=",
		"ip -4 addr show tailscale0",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("ordering drop-in missing %q in:\n%s", want, conf)
		}
	}
}

func TestNative_InstallAgent_WritesOrderingDropin(t *testing.T) {
	p, calls := nativeWithRunner(nil)
	if err := p.InstallAgent(context.Background(), JoinOpts{
		NodeIP: "100.97.82.15", ServerURL: "https://100.64.0.5:6443", Token: "K10tok",
		Pool: "linux", UseTailscale: true,
	}); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(*calls, "\n")
	for _, want := range []string{
		"mkdir -p /etc/systemd/system/k3s-agent.service.d",
		"base64 -d > /etc/systemd/system/k3s-agent.service.d/10-devx-tailscale.conf",
		"systemctl daemon-reload",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("a Tailscale agent install must write the ordering drop-in; missing %q in:\n%s", want, got)
		}
	}
}

func TestNative_InstallAgent_NoTailscaleNoDropin(t *testing.T) {
	p, calls := nativeWithRunner(nil)
	if err := p.InstallAgent(context.Background(), JoinOpts{
		NodeIP: "10.0.0.9", ServerURL: "https://10.0.0.5:6443", Token: "K10tok", Pool: "linux",
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(*calls, "\n"), "10-devx-tailscale.conf") {
		t.Error("no Tailscale → must not write the ordering drop-in")
	}
}

func TestNative_EnsureRuntime_InstallsISCSIForLonghorn(t *testing.T) {
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"tailscale ip -4":       "100.97.82.15",
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"iscsi-initiator-utils",
		"systemctl enable --now iscsid",
		"modprobe iscsi_tcp",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("EnsureRuntime missing %q in:\n%s", want, joined)
		}
	}
}

func TestNative_EnsureRuntime_InstallsOpenISCSIOnApt(t *testing.T) {
	// Neither rpm-ostree nor dnf present -> apt path; Debian's package is open-iscsi.
	p, calls := nativeWithRunner(map[string]string{
		"tailscale ip -4": "100.64.0.7",
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	if !strings.Contains(joined, "open-iscsi") {
		t.Errorf("apt path should install open-iscsi, got:\n%s", joined)
	}
	if strings.Contains(joined, "iscsi-initiator-utils") {
		t.Errorf("apt path must not use the Fedora package name, got:\n%s", joined)
	}
}

func TestNative_EnsureRuntime_FirewalldTrustsPodNetwork(t *testing.T) {
	// Regression: trusting only tailscale0 leaves pod-bridge traffic REJECTed
	// ("no route to host" pod-to-pod), which strands CSI volume attach.
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"firewall-cmd --state":  "running",
		"tailscale ip -4":       "100.97.82.15",
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"--add-interface=cni0",
		"--add-interface=flannel.1",
		"--add-source=10.42.0.0/16",
		"--add-source=10.43.0.0/16",
		"firewall-cmd --reload",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("firewalld trust missing %q in:\n%s", want, joined)
		}
	}
}

func TestNative_EnsureRuntime_SkipsFirewalldWhenAbsent(t *testing.T) {
	// No firewalld at all (not enabled, not running) => nothing to trust, skip.
	p, calls := nativeWithRunner(map[string]string{
		"tailscale ip -4": "100.64.0.7",
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, c := range *calls {
		if strings.Contains(c, "--zone=trusted") {
			t.Errorf("firewalld absent but trust command issued: %s", c)
		}
	}
}

func TestNative_EnsureRuntime_AppliesFirewalldTrustWhenEnabledButStopped(t *testing.T) {
	// Regression: on a fresh join firewalld can be momentarily stopped while the
	// host converges. Gating the pod-network trust on the live "running" state
	// silently skipped it, leaving pod egress REJECTed ("no route to host") until
	// a later reconcile -- this stranded external-dns on a freshly-joined node.
	// When firewalld is ENABLED we must start it and apply the trust regardless of
	// the momentary state. ("firewall-cmd --state" prints "not running" to stderr
	// when stopped, so with 2>/dev/null its stdout is empty -- modelled by omitting
	// the reply here.)
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree":          "/usr/bin/rpm-ostree",
		"systemctl is-enabled firewalld": "enabled",
		"tailscale ip -4":                "100.97.82.15",
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"systemctl start firewalld",
		"--add-source=10.42.0.0/16",
		"--add-source=10.43.0.0/16",
		"firewall-cmd --reload",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("enabled-but-stopped firewalld missing %q in:\n%s", want, joined)
		}
	}
}

func TestNative_Reconcile_HealsHostPrereqs(t *testing.T) {
	// Reconcile must converge existing nodes onto the full prereq set
	// (deps incl. iscsi, firewalld pod-network trust), not just re-install deps —
	// that is how already-joined nodes pick up these fixes on `cluster apply`.
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"firewall-cmd --state":  "running",
	})
	if err := p.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"iscsi-initiator-utils",
		"systemctl enable --now iscsid",
		"--add-source=10.42.0.0/16",
		"--add-interface=cni0",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Reconcile missing %q in:\n%s", want, joined)
		}
	}
}
