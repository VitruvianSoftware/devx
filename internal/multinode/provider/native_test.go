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
		"rpm -q":                "missing", // deps absent on the host → must be installed
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
	// rpm-ostree has no --allow-replacement flag (it aborts "Unknown option"), and
	// iscsi-initiator-utils is obsolete now that Longhorn is gone — neither must appear.
	for _, unwanted := range []string{"--allow-replacement", "iscsi-initiator-utils"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("EnsureRuntime must NOT emit %q in:\n%s", unwanted, joined)
		}
	}
}

func TestNative_EnsureRuntime_SilverblueSkipsPresentDeps(t *testing.T) {
	// On rpm-ostree, re-requesting an already-present package makes it try to UPDATE
	// (base change) which can't apply live on a stale base. So present deps must be
	// left alone — no `rpm-ostree install conntrack-tools/socat`.
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"tailscale ip -4":       "100.97.82.15",
		"rpm -q":                "present", // conntrack/socat already installed
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	if strings.Contains(joined, "install --apply-live --allow-inactive --idempotent conntrack-tools socat") {
		t.Errorf("present deps must not be re-installed on rpm-ostree:\n%s", joined)
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

func TestNative_EnsureRuntime_ConvergesStockFedora(t *testing.T) {
	// #403: the install skips the k3s-selinux RPM, so a STOCK Fedora host (SELinux
	// enforcing + nft iptables — the default, and what an in-place upgrade resets to)
	// breaks: systemd can't exec the k3s binary (203/EXEC Permission denied) and
	// iptables-restore fails ("COMMIT expected at line N"). EnsureRuntime must converge
	// the host to permissive SELinux + the iptables-legacy backend so the common case
	// (a freshly-installed or upgraded node) joins without hand-holding.
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"tailscale ip -4":       "100.86.106.117",
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"setenforce 0",       // SELinux permissive at runtime
		"SELINUX=permissive", // ...persisted across reboot
		"iptables-legacy",    // install the legacy backend
		"alternatives --set iptables /usr/sbin/iptables-legacy", // ...and select it
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("EnsureRuntime must converge stock Fedora: missing %q in:\n%s", want, joined)
		}
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

func TestNative_DepInstall_RpmOstreeNoBogusReplacementFlag(t *testing.T) {
	// Regression: rpm-ostree has NO --allow-replacement flag — it aborts with
	// "Unknown option --allow-replacement" (and --force-replacefiles errors with
	// "Non-local fileoverrides not implemented"). The install must use only valid
	// flags; the "packages would be changed" abort is avoided by only ever passing
	// genuinely-missing packages (see missingPkgs), not by an override flag.
	got := depInstallCmd("rpm-ostree", []string{"conntrack-tools"})
	for _, bad := range []string{"--allow-replacement", "--force-replacefiles"} {
		if strings.Contains(got, bad) {
			t.Errorf("rpm-ostree install must not use %q, got %q", bad, got)
		}
	}
	for _, want := range []string{"--apply-live", "--allow-inactive", "--idempotent"} {
		if !strings.Contains(got, want) {
			t.Errorf("rpm-ostree install must keep valid flag %q, got %q", want, got)
		}
	}
}

func TestNative_EnsureRuntime_PersistsTailscaledEnv(t *testing.T) {
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"tailscale ip -4":       "100.97.82.15",
	})
	if _, err := p.EnsureRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")

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
	// Regression: --add-interface and --add-source must NOT share one firewall-cmd
	// call — newer firewall-cmd rejects "--add-source not allowed with --add-interface".
	if strings.Contains(joined, "--add-interface=flannel.1 --add-source") {
		t.Errorf("interfaces and sources must be separate firewall-cmd calls:\n%s", joined)
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
	// Reconcile must converge already-joined nodes onto the host prereq set
	// (firewalld pod-network trust, etc.), not only on first join — that is how
	// existing nodes pick up these fixes on `cluster apply`.
	p, calls := nativeWithRunner(map[string]string{
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"firewall-cmd --state":  "running",
	})
	if err := p.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"--add-source=10.42.0.0/16",
		"--add-interface=cni0",
		"firewall-cmd --reload",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Reconcile missing %q in:\n%s", want, joined)
		}
	}
}
