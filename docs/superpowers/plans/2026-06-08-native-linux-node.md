# Native Linux Host Nodes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `devx cluster` add a node on a **native Linux host** (no Lima VM) — installing K3s directly over SSH — via a `NodeProvider` interface with Lima and Native implementations, selected by a per-node `kind` field. Motivating host: a Fedora Silverblue box on the tailnet, joined as an agent.

**Architecture:** Make `k3s.Manager`'s command execution pluggable (lima `limactl shell sudo` vs native `ssh sudo`), so the proven K3s install/join recipe is reused by both providers. A new `internal/multinode/provider` package defines `NodeProvider` (Lima wraps existing managers; Native adds host prep + direct install). `cluster.go` routes through `provider.For(node)`.

**Tech Stack:** Go 1.25, K3s (`get.k3s.io`), Tailscale, firewalld, rpm-ostree/dnf/apt, SSH (`internal/multinode/remote`).

---

## File Structure

- `internal/multinode/remote/remote.go` — add `RunSudo` (run a command as root on the host).
- `internal/multinode/k3s/k3s.go` — add a pluggable `exec` func + `sudo()` helper + `NewManagerWithExec`; route the 20 `LimaShellSudo` call-sites through `m.sudo(...)`. **Behavior-preserving.**
- `internal/multinode/config/config.go` + `validate.go` — `Kind`/`NodeIP` fields; relax VM validation for native.
- `internal/multinode/provider/provider.go` — `NodeProvider`, `JoinOpts`, `For` factory.
- `internal/multinode/provider/lima.go` — `LimaProvider`.
- `internal/multinode/provider/native.go` — `NativeProvider` + package-manager detection + command construction.
- `internal/multinode/provider/native_test.go` — injectable-runner unit tests.
- `internal/multinode/cluster/cluster.go` — route Join (and Init/Remove/Destroy/Status/Reconcile) through `provider.For`.

Shared types (exact names — keep consistent):

```go
// k3s.go
type execFunc func(ctx context.Context, command string) (string, error)

// provider/provider.go
type JoinOpts struct {
	NodeIP, ServerURL, Token, Pool, K3sVersion string
	TLSSANs                                     []string
	DisableServiceLB, UseTailscale, UseDocker   bool
}
type NodeProvider interface {
	EnsureRuntime(ctx context.Context) (nodeIP string, err error)
	InstallServer(ctx context.Context, o JoinOpts) error
	InstallAgent(ctx context.Context, o JoinOpts) error
	GetToken(ctx context.Context) (string, error)
	IsInstalled(ctx context.Context) (bool, error)
	WaitForReady(ctx context.Context, timeout time.Duration) error
	NodeStatus(ctx context.Context) (string, error)
	Kubeconfig(ctx context.Context, serverIP string) (string, error)
	Drain(ctx context.Context, nodeName string) error
	DeleteNode(ctx context.Context, nodeName string) error
	Uninstall(ctx context.Context, role string) error
	Destroy(ctx context.Context) error
	Reconcile(ctx context.Context) error
}
```

---

## Task 1: Pluggable K3s exec + `remote.RunSudo`

**Files:** Modify `internal/multinode/remote/remote.go`, `internal/multinode/k3s/k3s.go`; Test `internal/multinode/k3s/k3s_exec_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/multinode/k3s/k3s_exec_test.go (add MIT header from a sibling file)
package k3s

import (
	"context"
	"strings"
	"testing"
)

// A Manager built with NewManagerWithExec must route commands through the
// injected exec (used by the native provider), not limactl.
func TestManagerWithExec_RoutesThroughExec(t *testing.T) {
	var got []string
	m := NewManagerWithExec("fedora", func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		if strings.Contains(cmd, "node-token") {
			return "tok::123", nil
		}
		return "", nil
	})
	tok, err := m.GetToken(context.Background())
	if err != nil || tok != "tok::123" {
		t.Fatalf("GetToken via exec = %q,%v", tok, err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "/var/lib/rancher/k3s/server/node-token") {
		t.Errorf("exec should have run the node-token read; got %v", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/multinode/k3s/ -run WithExec`
Expected: FAIL — `undefined: NewManagerWithExec`.

- [ ] **Step 3: Implement** — in `remote.go` add:

```go
// RunSudo runs a command as root on the host via `sudo sh -c`.
func (r *Runner) RunSudo(ctx context.Context, command string) (string, error) {
	return r.Run(ctx, fmt.Sprintf("sudo sh -c %q", command))
}
```

In `k3s.go`: add the field + helper + constructor, and replace every
`m.runner.LimaShellSudo(ctx, m.vmName, X)` with `m.sudo(ctx, X)`:

```go
type execFunc func(ctx context.Context, command string) (string, error)

type Manager struct {
	runner *remote.Runner
	vmName string
	exec   execFunc
}

func NewManagerWithVM(runner *remote.Runner, vmName string) *Manager {
	if vmName == "" {
		vmName = defaultVMName
	}
	m := &Manager{runner: runner, vmName: vmName}
	m.exec = func(ctx context.Context, cmd string) (string, error) {
		return runner.LimaShellSudo(ctx, vmName, cmd) // lima: unchanged behavior
	}
	return m
}

// NewManagerWithExec builds a Manager that runs commands via the injected exec
// (e.g. ssh-sudo on a native Linux host). hostLabel is used only for log lines.
func NewManagerWithExec(hostLabel string, exec execFunc) *Manager {
	return &Manager{vmName: hostLabel, exec: exec}
}

func (m *Manager) sudo(ctx context.Context, cmd string) (string, error) { return m.exec(ctx, cmd) }
```

Then mechanically replace the 20 `m.runner.LimaShellSudo(ctx, m.vmName, …)` /
`m.runner.LimaShell(ctx, m.vmName, …)` call-sites with `m.sudo(ctx, …)`.
(`NewManager` keeps delegating to `NewManagerWithVM`.)

- [ ] **Step 4: Run tests** — `go test ./internal/multinode/k3s/ ./internal/multinode/...`
Expected: PASS (new test + all existing k3s tests unchanged — behavior-preserving).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/multinode/k3s/*.go internal/multinode/remote/remote.go
git add internal/multinode/k3s/ internal/multinode/remote/remote.go
git commit -m "refactor(cluster): make k3s.Manager command execution pluggable"
```

---

## Task 2: Config — `kind` / `nodeIP` + validation

**Files:** Modify `internal/multinode/config/config.go`, `internal/multinode/config/validate.go`; Test `internal/multinode/config/validate_test.go`

- [ ] **Step 1: Write the failing test (append to validate_test.go)**

```go
func TestValidate_NativeNodeSkipsVM(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "c"},
		Nodes: []NodeConfig{
			{Host: "cp", Role: "server", Pool: "p", VM: VMConfig{CPUs: 2, Memory: "4GiB", Disk: "40GiB"}},
			{Host: "fedora", Kind: "native", Role: "agent", Pool: "linux"}, // no VM fields
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("native node must not require vm fields: %v", err)
	}
}

func TestValidate_BadKind(t *testing.T) {
	cfg := &Config{
		Cluster: ClusterConfig{Name: "c"},
		Nodes:   []NodeConfig{{Host: "x", Kind: "windows", Role: "agent", Pool: "p"}},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("invalid kind must be rejected")
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/multinode/config/ -run 'Native|BadKind'` → FAIL (`Kind` undefined).

- [ ] **Step 3: Implement** — in `config.go` `NodeConfig` add:

```go
	Kind   string `yaml:"kind,omitempty"`   // "lima" (default) | "native"
	NodeIP string `yaml:"nodeIP,omitempty"` // native: override the auto-detected tailscale IP
```

Add a helper:

```go
// GetKind returns the node kind, defaulting to "lima".
func (n *NodeConfig) GetKind() string {
	if n.Kind == "" {
		return "lima"
	}
	return n.Kind
}
```

In `validate.go`, inside the per-node loop, add kind validation and make the VM
checks conditional (only the VM checks change; keep the rest):

```go
		switch n.GetKind() {
		case "lima":
			if n.VM.CPUs < 1 {
				errs = append(errs, fmt.Sprintf("nodes[%d].vm.cpus must be >= 1", i))
			}
			if n.VM.Memory == "" {
				errs = append(errs, fmt.Sprintf("nodes[%d].vm.memory is required", i))
			}
			if n.VM.Disk == "" {
				errs = append(errs, fmt.Sprintf("nodes[%d].vm.disk is required", i))
			}
		case "native":
			// no VM; the host runs K3s directly
		default:
			errs = append(errs, fmt.Sprintf("nodes[%d].kind %q must be 'lima' or 'native'", i, n.Kind))
		}
```

(Find the existing `vm.cpus`/`vm.memory`/`vm.disk` checks and move them into the
`case "lima"` block.)

- [ ] **Step 4: Run** — `go test ./internal/multinode/config/` → PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/multinode/config/*.go
git add internal/multinode/config/
git commit -m "feat(cluster): add node kind (lima|native) + nodeIP config"
```

---

## Task 3: `provider` package — interface, JoinOpts, factory

**Files:** Create `internal/multinode/provider/provider.go`; Test `internal/multinode/provider/provider_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/multinode/provider/provider_test.go (add MIT header)
package provider

import (
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

func TestFor_SelectsByKind(t *testing.T) {
	cfg := &config.Config{}
	if _, ok := For(config.NodeConfig{Host: "m", Role: "agent"}, cfg).(*LimaProvider); !ok {
		t.Error("default/empty kind must be a LimaProvider")
	}
	if _, ok := For(config.NodeConfig{Host: "fedora", Kind: "native", Role: "agent"}, cfg).(*NativeProvider); !ok {
		t.Error("kind=native must be a NativeProvider")
	}
}
```

- [ ] **Step 2: Run** — `go test ./internal/multinode/provider/` → FAIL (undefined).

- [ ] **Step 3: Implement** `provider.go` — the `NodeProvider` interface + `JoinOpts`
exactly as in the File Structure section above, plus:

```go
func For(node config.NodeConfig, cfg *config.Config) NodeProvider {
	if node.GetKind() == "native" {
		return NewNativeProvider(node, cfg)
	}
	return NewLimaProvider(node, cfg)
}
```

(Stub `NewLimaProvider`/`NewNativeProvider` to return the structs from Tasks 4–5;
implement those tasks before running cluster.go in Task 6.)

- [ ] **Step 4: Run** — `go test ./internal/multinode/provider/` → PASS (after Tasks 4–5 land the structs; sequence accordingly).

- [ ] **Step 5: Commit** — `git commit -m "feat(cluster): NodeProvider interface + factory"`

---

## Task 4: `LimaProvider` (behavior-preserving wrapper)

**Files:** Create `internal/multinode/provider/lima.go`

- [ ] **Step 1: Implement** — wraps the existing managers; each method delegates to
today's logic so macOS behavior is unchanged:

```go
type LimaProvider struct {
	node config.NodeConfig
	cfg  *config.Config
	run  *remote.Runner
	lima *lima.Manager
	k3s  *k3s.Manager
}

func NewLimaProvider(node config.NodeConfig, cfg *config.Config) *LimaProvider {
	run := util.NewRunner(node)
	return &LimaProvider{
		node: node, cfg: cfg, run: run,
		lima: lima.NewManager(run, node),
		k3s:  k3s.NewManagerWithVM(run, node.GetVMName()),
	}
}

func (p *LimaProvider) EnsureRuntime(ctx context.Context) (string, error) {
	if err := p.lima.Provision(ctx, p.cfg.Cluster.Docker.Enabled, p.cfg.Cluster.Mounts); err != nil {
		return "", err
	}
	ip, err := p.lima.GetBridgedIP(ctx)
	if err != nil {
		return "", err
	}
	if p.cfg.Cluster.Tailscale.Enabled && p.cfg.Cluster.Tailscale.AuthKey != "" {
		if tsIP, err := p.k3s.InstallTailscale(ctx, p.cfg.Cluster.Tailscale.AuthKey); err == nil {
			ip = tsIP
		} else {
			return "", err
		}
	}
	return ip, nil
}

func (p *LimaProvider) InstallServer(ctx context.Context, o JoinOpts) error {
	return p.k3s.JoinServer(ctx, o.NodeIP, o.ServerURL, o.Token, o.Pool, o.K3sVersion, o.TLSSANs, o.DisableServiceLB, o.UseTailscale, o.UseDocker)
}
func (p *LimaProvider) InstallAgent(ctx context.Context, o JoinOpts) error {
	return p.k3s.JoinAgent(ctx, o.NodeIP, o.ServerURL, o.Token, o.Pool, o.K3sVersion, o.UseTailscale, o.UseDocker)
}
func (p *LimaProvider) GetToken(ctx context.Context) (string, error)        { return p.k3s.GetToken(ctx) }
func (p *LimaProvider) IsInstalled(ctx context.Context) (bool, error)       { return p.k3s.IsInstalled(ctx) }
func (p *LimaProvider) WaitForReady(ctx context.Context, d time.Duration) error { return p.k3s.WaitForReady(ctx, d) }
func (p *LimaProvider) NodeStatus(ctx context.Context) (string, error)      { return p.k3s.GetNodeStatus(ctx) }
func (p *LimaProvider) Kubeconfig(ctx context.Context, ip string) (string, error) { return p.k3s.GetKubeconfig(ctx, ip) }
func (p *LimaProvider) Drain(ctx context.Context, n string) error           { return p.k3s.DrainNode(ctx, n) }
func (p *LimaProvider) DeleteNode(ctx context.Context, n string) error      { return p.k3s.DeleteNode(ctx, n) }
func (p *LimaProvider) Uninstall(ctx context.Context, role string) error    { return p.k3s.Uninstall(ctx, role) }
func (p *LimaProvider) Destroy(ctx context.Context) error {
	_ = p.k3s.Uninstall(ctx, p.node.Role)
	return p.lima.Destroy(ctx)
}
func (p *LimaProvider) Reconcile(ctx context.Context) error {
	_, err := p.run.LimaShellSudo(ctx, p.node.GetVMName(), fmt.Sprintf("apt-get update -qq && apt-get install -y -qq %s", lima.NodePackages))
	return err
}
```

- [ ] **Step 2: Build** — `go build ./internal/multinode/...` → ok.
- [ ] **Step 3: Commit** — `git commit -m "feat(cluster): LimaProvider wrapping existing managers"`

---

## Task 5: `NativeProvider` (Fedora/Silverblue) — the core new code

**Files:** Create `internal/multinode/provider/native.go`, `internal/multinode/provider/native_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// native_test.go (add MIT header)
package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

func nativeWithRunner(t *testing.T, replies map[string]string) (*NativeProvider, *[]string) {
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
	p, calls := nativeWithRunner(t, map[string]string{
		"ID=":          "ID=fedora\nVARIANT_ID=silverblue",
		"command -v rpm-ostree": "/usr/bin/rpm-ostree",
		"firewall-cmd --state":  "running",
		"tailscale ip -4":       "100.97.82.15",
	})
	ip, err := p.EnsureRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ip != "100.97.82.15" {
		t.Errorf("node ip = %q, want tailscale ip", ip)
	}
	joined := strings.Join(*calls, "\n")
	for _, want := range []string{
		"rpm-ostree install",                       // immutable dep install
		"--apply-live",                             // no reboot
		"conntrack-tools socat",                    // the deps
		"firewall-cmd --permanent --zone=trusted --add-interface=tailscale0",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("EnsureRuntime missing %q in:\n%s", want, joined)
		}
	}
}

func TestNative_InstallAgent_Recipe(t *testing.T) {
	p, calls := nativeWithRunner(t, nil)
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
```

- [ ] **Step 2: Run** — `go test ./internal/multinode/provider/ -run Native` → FAIL.

- [ ] **Step 3: Implement** `native.go`:

```go
type NativeProvider struct {
	node config.NodeConfig
	cfg  *config.Config
	run  execFuncNative // injectable; default = ssh sudo
	k3s  *k3s.Manager
}

type execFuncNative func(ctx context.Context, command string) (string, error)

func NewNativeProvider(node config.NodeConfig, cfg *config.Config) *NativeProvider {
	r := util.NewRunner(node)
	run := func(ctx context.Context, cmd string) (string, error) { return r.RunSudo(ctx, cmd) }
	return &NativeProvider{
		node: node, cfg: cfg, run: run,
		k3s:  k3s.NewManagerWithExec(node.Host, func(ctx context.Context, cmd string) (string, error) { return run(ctx, cmd) }),
	}
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

func (p *NativeProvider) EnsureRuntime(ctx context.Context) (string, error) {
	// 1. deps required by K3s (conntrack) and devx bridge (socat).
	pm := p.detectPkgManager(ctx)
	if _, err := p.run(ctx, depInstallCmd(pm, []string{"conntrack-tools", "socat"})); err != nil {
		return "", fmt.Errorf("[%s] installing deps: %w", p.node.Host, err)
	}
	// 2. trust the tailnet interface in firewalld (if active).
	if out, _ := p.run(ctx, "firewall-cmd --state 2>/dev/null || true"); strings.Contains(out, "running") {
		_, _ = p.run(ctx, "firewall-cmd --permanent --zone=trusted --add-interface=tailscale0 && firewall-cmd --reload")
	}
	// 3. ensure tailscale up (host already on tailnet in our case; bring up if a key is set).
	if p.cfg.Cluster.Tailscale.Enabled && p.cfg.Cluster.Tailscale.AuthKey != "" {
		_, _ = p.run(ctx, fmt.Sprintf("command -v tailscale && (tailscale status >/dev/null 2>&1 || tailscale up --authkey=%s --accept-routes)", p.cfg.Cluster.Tailscale.AuthKey))
	}
	// 4. node IP: explicit override, else the tailscale IP.
	if p.node.NodeIP != "" {
		return p.node.NodeIP, nil
	}
	out, err := p.run(ctx, "tailscale ip -4")
	if err != nil {
		return "", fmt.Errorf("[%s] tailscale ip: %w", p.node.Host, err)
	}
	return strings.TrimSpace(out), nil
}

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
		sans := ""
		for _, s := range o.TLSSANs {
			sans += fmt.Sprintf(" --tls-san=%s", s)
		}
		lb := ""
		if o.DisableServiceLB {
			lb = " --disable servicelb"
		}
		return fmt.Sprintf(`curl -sfL https://get.k3s.io | %s INSTALL_K3S_EXEC="server" sh -s - --server=%s --node-name=%s --node-ip=%s --advertise-address=%s%s%s%s --node-label=pool=%s`,
			common, o.ServerURL, p.node.Host, o.NodeIP, o.NodeIP, sans, flannel, lb, o.Pool)
	}
	return fmt.Sprintf(`curl -sfL https://get.k3s.io | %s K3S_URL=%q sh -s - agent --node-name=%s --node-ip=%s%s --node-label=pool=%s`,
		common, o.ServerURL, p.node.Host, o.NodeIP, flannel, o.Pool)
}

func (p *NativeProvider) InstallAgent(ctx context.Context, o JoinOpts) error {
	if inst, _ := p.IsInstalled(ctx); inst {
		return nil
	}
	_, err := p.run(ctx, p.installScript("agent", o))
	return err
}
func (p *NativeProvider) InstallServer(ctx context.Context, o JoinOpts) error {
	if inst, _ := p.IsInstalled(ctx); inst {
		return nil
	}
	_, err := p.run(ctx, p.installScript("server", o))
	return err
}

// The rest delegate to the exec-backed k3s.Manager (same commands as Lima).
func (p *NativeProvider) GetToken(ctx context.Context) (string, error)            { return p.k3s.GetToken(ctx) }
func (p *NativeProvider) IsInstalled(ctx context.Context) (bool, error)           { return p.k3s.IsInstalled(ctx) }
func (p *NativeProvider) WaitForReady(ctx context.Context, d time.Duration) error { return p.k3s.WaitForReady(ctx, d) }
func (p *NativeProvider) NodeStatus(ctx context.Context) (string, error)          { return p.k3s.GetNodeStatus(ctx) }
func (p *NativeProvider) Kubeconfig(ctx context.Context, ip string) (string, error) { return p.k3s.GetKubeconfig(ctx, ip) }
func (p *NativeProvider) Drain(ctx context.Context, n string) error               { return p.k3s.DrainNode(ctx, n) }
func (p *NativeProvider) DeleteNode(ctx context.Context, n string) error          { return p.k3s.DeleteNode(ctx, n) }
func (p *NativeProvider) Uninstall(ctx context.Context, role string) error        { return p.k3s.Uninstall(ctx, role) }
func (p *NativeProvider) Destroy(ctx context.Context) error                       { return p.k3s.Uninstall(ctx, p.node.Role) }
func (p *NativeProvider) Reconcile(ctx context.Context) error {
	pm := p.detectPkgManager(ctx)
	_, err := p.run(ctx, depInstallCmd(pm, []string{"conntrack-tools", "socat"}))
	return err
}
```

> NOTE: `k3s.NewManagerWithExec`'s exec is `execFunc` (Task 1). Adapt the `run`
> closure type as needed so both compile (one `func(ctx, string)(string,error)`).

- [ ] **Step 4: Run** — `go test ./internal/multinode/provider/` → PASS.
- [ ] **Step 5: Commit** — `git commit -m "feat(cluster): NativeProvider for direct K3s on Linux hosts"`

---

## Task 6: Route `cluster.go` Join (and the rest) through the provider

**Files:** Modify `internal/multinode/cluster/cluster.go`

- [ ] **Step 1: Implement** — replace the per-node `util.NewRunner` + `lima.NewManager`
+ `k3s.NewManagerWithVM` blocks in **Join** (then Init/Remove/Destroy/Status/Reconcile)
with `prov := provider.For(node, cfg)` and the interface methods. The `Join` loop
becomes (preserving dry-run + ordering):

```go
for _, node := range cfg.Nodes {
	prov := provider.For(node, cfg)
	if inst, _ := prov.IsInstalled(ctx); inst {
		continue
	}
	if dryRun {
		fmt.Printf("  [%s] Would join as %s (kind=%s, pool=%s)\n", node.Host, node.Role, node.GetKind(), node.Pool)
		continue
	}
	nodeIP, err := prov.EnsureRuntime(ctx)
	if err != nil {
		return fmt.Errorf("[%s] ensuring runtime: %w", node.Host, err)
	}
	o := provider.JoinOpts{
		NodeIP: nodeIP, ServerURL: serverURL, Token: token, Pool: node.Pool,
		K3sVersion: cfg.Cluster.K3sVersion, TLSSANs: []string{nodeIP},
		DisableServiceLB: cfg.Cluster.MetalLB.Enabled, UseTailscale: cfg.Cluster.Tailscale.Enabled,
		UseDocker: cfg.Cluster.Docker.Enabled,
	}
	if node.Role == "server" {
		err = prov.InstallServer(ctx, o)
	} else {
		err = prov.InstallAgent(ctx, o)
	}
	if err != nil {
		return fmt.Errorf("[%s] joining as %s: %w", node.Host, node.Role, err)
	}
	slog.Info("node joined", "host", node.Host, "role", node.Role, "kind", node.GetKind())
}
```

The "find a healthy server for token + serverURL" block at the top of `Join`
similarly uses `provider.For(n, cfg)` + `IsInstalled`/`GetToken`; serverURL is the
healthy server's `EnsureRuntime`-less IP — reuse the existing IP discovery
(GetBridgedIP via a LimaProvider helper, or for an all-checked-in cluster, read
the token from any installed server via `prov.GetToken`).

- [ ] **Step 2: Build + existing tests** — `go build ./... && go test ./internal/multinode/...` → ok/PASS.
- [ ] **Step 3: Commit** — `git commit -m "feat(cluster): route join through NodeProvider"`

---

## Task 7: Gated integration — join the real `fedora` host

**Files:** Create `internal/multinode/provider/native_integration_test.go` (`//go:build integration`)

- [ ] **Step 1: Write the gated test** — joins `fedora` (tailnet) as an agent against
the live cluster's server URL/token, asserts it reaches `Ready`, then removes it.
Skips unless `ssh fedora` works and `DEVX_NATIVE_ITEST_SERVER`/`_TOKEN` env are set.

```go
//go:build integration

package provider

// Run: DEVX_NATIVE_ITEST_SERVER=https://<vm-tailnet-ip>:6443 DEVX_NATIVE_ITEST_TOKEN=… \
//   go test -tags integration ./internal/multinode/provider/ -run NativeIntegration -timeout 15m
// (env left to the operator; this exercises EnsureRuntime + InstallAgent + WaitForReady
//  against the real Fedora host, then Uninstall.)
```

- [ ] **Step 2: Commit** — `git commit -m "test(cluster): gated native-join integration test"`

---

## Task 8: Final verification

- [ ] **Step 1: Preflight** — `go build ./... && go test ./internal/multinode/... ./cmd/ && golangci-lint run ./internal/multinode/...`
- [ ] **Step 2: Live verify** — add the `fedora` node to `cluster.yaml`
  (`kind: native, role: agent, pool: linux`), run `devx cluster join`, and confirm
  `kubectl get nodes` shows `fedora` `Ready`.
- [ ] **Step 3: Open PR** for review (leave unmerged per the merge-review policy).

---

## Self-Review

- **Spec coverage:** §4 interface → Tasks 1,3; LimaProvider → Task 4; NativeProvider
  behavior (§5) → Task 5; config (§6) → Task 2; cluster routing (§4) → Task 6;
  testing (§8) → Tasks 1,2,3,5,7; live verify (§goal) → Task 8. Out-of-scope (§9)
  honored (Permissive-only, --apply-live, ACL field check). Covered.
- **Placeholder scan:** Task 6's healthy-server discovery is described, not fully
  coded, because it depends on the existing IP-discovery the refactor preserves —
  the implementer keeps that block and swaps the manager calls for provider calls;
  flagged explicitly. No `TBD`/`add error handling` placeholders in code steps.
- **Type consistency:** `NodeProvider`, `JoinOpts`, `For`, `NewLimaProvider`,
  `NewNativeProvider`, `execFunc`, `NewManagerWithExec`, `RunSudo`, `GetKind`,
  `depInstallCmd` — names consistent across tasks. One hazard noted in Task 5: the
  k3s exec type (`execFunc`) vs the native `run` closure must unify to a single
  `func(ctx, string)(string,error)` signature.
