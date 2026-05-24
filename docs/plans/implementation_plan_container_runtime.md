# `runtime: container` Deployer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `runtime: container` services actually deploy and run in `devx up` — today the `RuntimeContainer` enum exists but `Execute` has no dispatch case, so a container service silently falls through to the host path and never runs as a container.

**Architecture:** Mirror the existing k8s/cloud deployer pattern in `internal/orchestrator`. Add a `container:` config block (pre-built `image:` XOR a `build:` block), a `ContainerNodeConfig` carried on the `Node`, and a `startContainerNode` that resolves the provider runtime, optionally builds the image via `internal/image`, runs it detached (`<rt> run -d --name devx-svc-<name> ...`), and registers `rm -f` cleanup. Add a `RuntimeContainer` dispatch case so it is invoked. The generic HTTP/TCP healthcheck applies automatically (same as host), because container nodes are not special-cased like k8s/cloud.

**Tech Stack:** Go 1.25.5, cobra, gopkg.in/yaml.v3, `provider.ContainerRuntime` (podman/docker/nerdctl wrapper), `internal/image` (build), `internal/network` (port resolution). Tests: stdlib `testing` (no testify in this repo).

**Prerequisite context for the engineer:**
- Containers in devx are named `devx-<kind>-<name>` and labeled `managed-by=devx`, run detached with `run -d --name … -p … --restart unless-stopped`, and removed with `rm -f` (see [internal/mock/server.go:90-104](../../internal/mock/server.go) and [internal/testing/ephemeral.go:88-94](../../internal/testing/ephemeral.go)).
- The provider runtime is obtained with `provider.Resolve(name) (provider.VMProvider, provider.ContainerRuntime, error)` (see [cmd/vm.go:72](../../cmd/vm.go)). `ContainerRuntime` has `CommandContext(ctx, args...) *exec.Cmd` and `Exec(args...) (string, error)` ([internal/provider/provider.go:88-105](../../internal/provider/provider.go)).
- Image building: `image.Build(ctx, rt, baseDir string, spec image.Spec, ref string) error` ([internal/image/build.go:61](../../internal/image/build.go)); `image.Spec{Name, Context, Dockerfile, Tag, Platforms}` ([internal/image/image.go:39](../../internal/image/image.go)).
- Port resolution: `network.ResolvePort(port int) (actual int, shifted bool, warning string)` (used at [cmd/up.go](../../cmd/up.go) and [kubernetes_portforward.go](../../internal/orchestrator/kubernetes_portforward.go)).
- Run tests with `go test ./...` (CI uses `go test -v -race ./...`).

---

### Task 1: Add the `container:` config schema

**Files:**
- Modify: `cmd/devxconfig.go` (add two structs + one field on `DevxConfigService` at line 183-197)
- Test: `cmd/devxconfig_test.go`

- [ ] **Step 1: Write the failing test**

Add to `cmd/devxconfig_test.go`:

```go
func TestResolveConfig_ContainerRuntime_Image(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: test-proj
services:
  - name: api
    runtime: container
    container:
      image: myorg/api:dev
    port: 8080
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(yamlPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := cfg.Services[0]
	if svc.Runtime != "container" {
		t.Errorf("expected runtime container, got %q", svc.Runtime)
	}
	if svc.Container == nil {
		t.Fatal("expected Container config, got nil")
	}
	if svc.Container.Image != "myorg/api:dev" {
		t.Errorf("expected image myorg/api:dev, got %q", svc.Container.Image)
	}
	if svc.Container.Build != nil {
		t.Errorf("expected nil Build, got %+v", svc.Container.Build)
	}
}

func TestResolveConfig_ContainerRuntime_Build(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(yamlPath, []byte(`
name: test-proj
services:
  - name: web
    runtime: container
    container:
      build:
        context: ./web
        dockerfile: Dockerfile.dev
        tag: local
    port: 3000
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := resolveConfig(yamlPath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b := cfg.Services[0].Container.Build
	if b == nil {
		t.Fatal("expected Build config, got nil")
	}
	if b.Context != "./web" || b.Dockerfile != "Dockerfile.dev" || b.Tag != "local" {
		t.Errorf("unexpected build config: %+v", b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestResolveConfig_ContainerRuntime -v`
Expected: FAIL — compile error `svc.Container undefined (type DevxConfigService has no field or method Container)`.

- [ ] **Step 3: Add the structs and field**

In `cmd/devxconfig.go`, add these struct definitions (place them just above `DevxConfigService` at line 183):

```go
// DevxConfigContainerBuild describes how to build a runtime: container image
// from a local build context (compose-style `build:`). Reuses the same shape as
// kubernetes.images but without a Name (the image is local, tagged after the service).
type DevxConfigContainerBuild struct {
	Context    string   `yaml:"context,omitempty"`    // build context dir (default ".")
	Dockerfile string   `yaml:"dockerfile,omitempty"` // Dockerfile path relative to context (default "Dockerfile")
	Tag        string   `yaml:"tag,omitempty"`        // image tag (default "dev")
	Platforms  []string `yaml:"platforms,omitempty"`  // target platforms; empty = builder default
}

// DevxConfigServiceContainer defines a runtime: container service — a long-lived
// container devx runs in the VM. Exactly one of Image or Build must be set.
type DevxConfigServiceContainer struct {
	Image string                    `yaml:"image,omitempty"` // pre-built image to run (mutually exclusive with build)
	Build *DevxConfigContainerBuild `yaml:"build,omitempty"` // build from local context (mutually exclusive with image)
	Args  []string                  `yaml:"args,omitempty"`  // extra flags appended to `<runtime> run`
}
```

Then add this field to `DevxConfigService` (after the `CloudRun` field, line 195):

```go
	Container       *DevxConfigServiceContainer       `yaml:"container,omitempty"`        // runtime: container deploy spec
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestResolveConfig_ContainerRuntime -v`
Expected: PASS (both subtests).

- [ ] **Step 5: Commit**

```bash
git add cmd/devxconfig.go cmd/devxconfig_test.go
git commit -m "feat(config): add runtime: container schema (image | build)"
```

---

### Task 2: Add `ContainerNodeConfig` and map it in `up.go`

**Files:**
- Modify: `internal/orchestrator/dag.go` (add `ContainerNodeConfig` struct near line 64-102; add `Container` + `containerStarted` fields to `Node` at 145-180)
- Modify: `cmd/up.go` (add a mapping helper + wire it into the service loop ~line 252-394, and into the `dag.AddNode` call)
- Test: `cmd/up_container_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `cmd/up_container_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/VitruvianSoftware/devx/internal/image"
)

func TestToContainerNodeConfig_Image(t *testing.T) {
	svc := DevxConfigService{
		Name:      "api",
		Runtime:   "container",
		Container: &DevxConfigServiceContainer{Image: "myorg/api:dev", Args: []string{"--cap-add=NET_ADMIN"}},
	}
	cfg := toContainerNodeConfig(svc, "lima")
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Image != "myorg/api:dev" {
		t.Errorf("image = %q", cfg.Image)
	}
	if cfg.Build != nil {
		t.Errorf("expected nil build, got %+v", cfg.Build)
	}
	if cfg.ProviderName != "lima" {
		t.Errorf("provider = %q", cfg.ProviderName)
	}
	if len(cfg.Args) != 1 || cfg.Args[0] != "--cap-add=NET_ADMIN" {
		t.Errorf("args = %v", cfg.Args)
	}
}

func TestToContainerNodeConfig_Build(t *testing.T) {
	svc := DevxConfigService{
		Name:    "web",
		Runtime: "container",
		Container: &DevxConfigServiceContainer{
			Build: &DevxConfigContainerBuild{Context: "./web", Dockerfile: "Dockerfile.dev", Tag: "local", Platforms: []string{"linux/arm64"}},
		},
	}
	cfg := toContainerNodeConfig(svc, "lima")
	want := &image.Spec{Name: "web", Context: "./web", Dockerfile: "Dockerfile.dev", Tag: "local", Platforms: []string{"linux/arm64"}}
	if cfg.Build == nil {
		t.Fatal("expected build spec, got nil")
	}
	if cfg.Build.Name != want.Name || cfg.Build.Context != want.Context ||
		cfg.Build.Dockerfile != want.Dockerfile || cfg.Build.Tag != want.Tag {
		t.Errorf("build spec = %+v, want %+v", cfg.Build, want)
	}
}

func TestToContainerNodeConfig_NilWhenNoBlock(t *testing.T) {
	svc := DevxConfigService{Name: "api", Runtime: "container"}
	if cfg := toContainerNodeConfig(svc, "lima"); cfg != nil {
		t.Errorf("expected nil when container block absent, got %+v", cfg)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/ -run TestToContainerNodeConfig -v`
Expected: FAIL — `undefined: toContainerNodeConfig` and `orchestrator.ContainerNodeConfig`.

- [ ] **Step 3a: Add the orchestrator config struct + Node fields**

In `internal/orchestrator/dag.go`, add this struct after `CloudRunNodeConfig` (after line 102):

```go
// ContainerNodeConfig holds runtime: container configuration for a DAG node:
// devx runs a long-lived container in the VM via the provider runtime. Exactly
// one of Image or Build is set.
type ContainerNodeConfig struct {
	Image        string      // pre-built image to run (empty when Build is set)
	Build        *image.Spec // build-from-context spec (nil when Image is set)
	Args         []string    // extra flags appended to `<runtime> run`
	ProviderName string      // container provider to resolve the runtime (e.g. "lima")
}
```

In the `Node` struct (line 145-180), add these fields after the `CloudRun`/`crDeployed` fields:

```go
	// Container deploy field (runtime: container) + whether we started it, for cleanup
	Container        *ContainerNodeConfig
	containerStarted bool
```

(`image` is already imported in dag.go — `KubeNodeConfig.Images` is `[]image.Spec`.)

- [ ] **Step 3b: Add the mapping helper and wire it into `up.go`**

In `cmd/up.go`, add this helper function (near the other top-level helpers, e.g. above `mustGetwd` at line 468):

```go
// toContainerNodeConfig maps a runtime: container service to its orchestrator
// config. Returns nil if the service has no `container:` block.
func toContainerNodeConfig(svc DevxConfigService, providerName string) *orchestrator.ContainerNodeConfig {
	if svc.Container == nil {
		return nil
	}
	cfg := &orchestrator.ContainerNodeConfig{
		Image:        svc.Container.Image,
		Args:         svc.Container.Args,
		ProviderName: providerName,
	}
	if b := svc.Container.Build; b != nil {
		cfg.Build = &image.Spec{
			Name:       svc.Name,
			Context:    b.Context,
			Dockerfile: b.Dockerfile,
			Tag:        b.Tag,
			Platforms:  b.Platforms,
		}
	}
	return cfg
}
```

Ensure `cmd/up.go` imports `"github.com/VitruvianSoftware/devx/internal/image"` (it already references `image.Spec` in the kubernetes mapping, so the import exists).

In the service loop, add a `containerCfg` variable alongside the other config vars (near line 258, with `kubeCfg`/`crCfg`):

```go
			var containerCfg *orchestrator.ContainerNodeConfig
```

In the `switch svc.Runtime` block, replace the bare `case "container":` (line 265-266, currently just `rt = orchestrator.RuntimeContainer`) with:

```go
			case "container":
				rt = orchestrator.RuntimeContainer
				containerCfg = toContainerNodeConfig(svc, resolveProviderName())
```

Finally, add `Container: containerCfg,` to the `dag.AddNode(&orchestrator.Node{...})` literal (alongside `Kube: kubeCfg,` and `CloudRun: crCfg,` near line 388):

```go
				Container:    containerCfg,
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/ -run TestToContainerNodeConfig -v`
Expected: PASS (all three subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/dag.go cmd/up.go cmd/up_container_test.go
git commit -m "feat(orchestrator): add ContainerNodeConfig and up.go mapping"
```

---

### Task 3: `startContainerNode` (pre-built image path) + dispatch

**Files:**
- Create: `internal/orchestrator/container_node.go`
- Modify: `internal/orchestrator/dag.go` (add `case RuntimeContainer:` to the `Execute` switch at line 337-347)
- Test: `internal/orchestrator/container_node_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `internal/orchestrator/container_node_test.go`:

```go
package orchestrator

import (
	"reflect"
	"testing"
)

func TestContainerNodeName(t *testing.T) {
	if got := containerNodeName("api"); got != "devx-svc-api" {
		t.Errorf("containerNodeName = %q, want devx-svc-api", got)
	}
}

func TestContainerRunArgs_ImageWithPortEnvCommand(t *testing.T) {
	c := &ContainerNodeConfig{Image: "myorg/api:dev", Args: []string{"--cap-add=NET_ADMIN"}}
	got := containerRunArgs(c, "devx-svc-api", "myorg/api:dev", 18080, 8080,
		map[string]string{"LOG_LEVEL": "debug"}, []string{"./api", "--dev"})
	want := []string{
		"run", "-d",
		"--name", "devx-svc-api",
		"--label", "managed-by=devx",
		"--label", "devx-service=api",
		"--restart", "unless-stopped",
		"-p", "18080:8080",
		"-e", "LOG_LEVEL=debug",
		"--cap-add=NET_ADMIN",
		"myorg/api:dev",
		"./api", "--dev",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("containerRunArgs =\n  %v\nwant\n  %v", got, want)
	}
}

func TestContainerRunArgs_NoPortNoEnvNoCommand(t *testing.T) {
	c := &ContainerNodeConfig{Image: "nginx:1.27"}
	got := containerRunArgs(c, "devx-svc-web", "nginx:1.27", 0, 0, nil, nil)
	want := []string{
		"run", "-d",
		"--name", "devx-svc-web",
		"--label", "managed-by=devx",
		"--label", "devx-service=web",
		"--restart", "unless-stopped",
		"nginx:1.27",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("containerRunArgs =\n  %v\nwant\n  %v", got, want)
	}
}
```

Note: `devx-service` label value is derived by trimming the `devx-svc-` prefix from the container name; `containerRunArgs` takes the container name and strips the prefix for the label.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run 'TestContainerNodeName|TestContainerRunArgs' -v`
Expected: FAIL — `undefined: containerNodeName`, `undefined: containerRunArgs`.

- [ ] **Step 3: Implement `container_node.go` (image path) + dispatch**

Create `internal/orchestrator/container_node.go`:

```go
package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/network"
	"github.com/VitruvianSoftware/devx/internal/provider"
)

const containerNodeBase = "devx-svc"

// containerNodeName returns the VM container name for a service node.
func containerNodeName(service string) string {
	return fmt.Sprintf("%s-%s", containerNodeBase, service)
}

// containerRunArgs builds the `<runtime> run` argument list. Pure function,
// separated for unit testing. localPort==0 means no port mapping.
func containerRunArgs(c *ContainerNodeConfig, name, ref string, localPort, containerPort int, env map[string]string, command []string) []string {
	service := strings.TrimPrefix(name, containerNodeBase+"-")
	args := []string{
		"run", "-d",
		"--name", name,
		"--label", "managed-by=devx",
		"--label", "devx-service=" + service,
		"--restart", "unless-stopped",
	}
	if localPort > 0 && containerPort > 0 {
		args = append(args, "-p", fmt.Sprintf("%d:%d", localPort, containerPort))
	}
	// Deterministic env ordering for stable args (and stable tests).
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, env[k]))
	}
	args = append(args, c.Args...)
	args = append(args, ref)
	args = append(args, command...)
	return args
}

// startContainerNode runs a runtime: container service as a long-lived container
// in the VM via the provider runtime (skaffold-class local container). Like the
// other deployers it shells out — here to `<runtime> run` — and registers a
// cleanup that removes the container on shutdown. The generic HTTP/TCP
// healthcheck still applies (container nodes are not special-cased).
func startContainerNode(ctx context.Context, n *Node) error {
	c := n.Container
	if c == nil {
		return fmt.Errorf("service %q has runtime: container but no container config is set", n.Name)
	}
	if (c.Image == "") == (c.Build == nil) {
		return fmt.Errorf("service %q: exactly one of container.image or container.build must be set", n.Name)
	}

	_, rt, err := provider.Resolve(c.ProviderName)
	if err != nil {
		return fmt.Errorf("service %q: resolving container runtime: %w", n.Name, err)
	}

	ref := c.Image // build path overrides this in a later task

	name := containerNodeName(n.Name)
	// Remove any stale container with the same name so re-running `up` is idempotent.
	_, _ = rt.Exec("rm", "-f", name)

	localPort, containerPort := 0, 0
	if n.Port > 0 {
		containerPort = n.Port // the port the container listens on (from devx.yaml)
		actual, _, _ := network.ResolvePort(n.Port)
		n.Port = actual // local/host port; the healthcheck connects here
		localPort = actual
	}

	fmt.Printf("  🐳 Running %s → container %s (%s)\n", n.Name, name, ref)
	cmd := rt.CommandContext(ctx, containerRunArgs(c, name, ref, localPort, containerPort, n.Env, n.Command)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("running container for %q failed: %w\n%s", n.Name, err, strings.TrimSpace(string(out)))
	}
	n.containerStarted = true
	fmt.Printf("  ✅ %s running\n", n.Name)
	return nil
}
```

In `internal/orchestrator/dag.go`, add a case to the `Execute` switch (after the `RuntimeCloud` case, before `default:`, around line 351):

```go
					case RuntimeContainer:
						if err := startContainerNode(ctx, n); err != nil {
							errCh <- fmt.Errorf("failed to start container service %q: %w", n.Name, err)
							return
						}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator/ -run 'TestContainerNodeName|TestContainerRunArgs' -v`
Expected: PASS.
Then `go build ./...` — Expected: builds clean (dispatch case compiles).

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/container_node.go internal/orchestrator/dag.go internal/orchestrator/container_node_test.go
git commit -m "feat(orchestrator): run runtime: container services (pre-built image)"
```

---

### Task 4: `startContainerNode` build-from-context path

**Files:**
- Modify: `internal/orchestrator/container_node.go` (add `containerImageRef` + wire the build branch)
- Test: `internal/orchestrator/container_node_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/orchestrator/container_node_test.go`:

```go
import "github.com/VitruvianSoftware/devx/internal/image" // add to existing import block

func TestContainerImageRef_BuildDefaultsTag(t *testing.T) {
	ref := containerImageRef("web", &image.Spec{Name: "web", Context: "."})
	if ref != "devx-svc-web:dev" {
		t.Errorf("ref = %q, want devx-svc-web:dev", ref)
	}
}

func TestContainerImageRef_BuildExplicitTag(t *testing.T) {
	ref := containerImageRef("web", &image.Spec{Name: "web", Tag: "local"})
	if ref != "devx-svc-web:local" {
		t.Errorf("ref = %q, want devx-svc-web:local", ref)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run TestContainerImageRef -v`
Expected: FAIL — `undefined: containerImageRef`.

- [ ] **Step 3: Implement the build branch**

In `internal/orchestrator/container_node.go`, add the import `"github.com/VitruvianSoftware/devx/internal/image"` and this helper:

```go
// containerImageRef computes the local image tag for a built container service.
func containerImageRef(service string, spec *image.Spec) string {
	tag := spec.Tag
	if tag == "" {
		tag = "dev"
	}
	return fmt.Sprintf("%s-%s:%s", containerNodeBase, service, tag)
}
```

Then replace the `ref := c.Image // build path overrides this in a later task` line in `startContainerNode` with:

```go
	ref := c.Image
	if c.Build != nil {
		ref = containerImageRef(n.Name, c.Build)
		baseDir := n.Dir
		if baseDir == "" {
			baseDir = "."
		}
		fmt.Printf("  🔨 Building %s image %s\n", n.Name, ref)
		if err := image.Build(ctx, rt, baseDir, *c.Build, ref); err != nil {
			return fmt.Errorf("building image for %q: %w", n.Name, err)
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator/ -run TestContainerImageRef -v`
Expected: PASS.
Then `go build ./...` — Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/container_node.go internal/orchestrator/container_node_test.go
git commit -m "feat(orchestrator): build runtime: container images from context"
```

---

### Task 5: Cleanup registration (remove container on shutdown)

**Files:**
- Modify: `internal/orchestrator/container_node.go` (add `removeContainerNode`)
- Modify: `internal/orchestrator/dag.go` (call it in `cleanupFn` at line 290-320)
- Test: `internal/orchestrator/container_node_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/orchestrator/container_node_test.go`:

```go
func TestRemoveContainerNode_NoopWhenNotStarted(t *testing.T) {
	// removeContainerNode must be a safe no-op when the container was never started,
	// so cleanup never resolves a provider for a node that didn't run.
	n := &Node{Name: "api", Container: &ContainerNodeConfig{Image: "x", ProviderName: "definitely-not-a-provider"}}
	// containerStarted is false → must return nil without resolving the runtime.
	if err := removeContainerNode(n); err != nil {
		t.Errorf("expected no-op nil, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/orchestrator/ -run TestRemoveContainerNode -v`
Expected: FAIL — `undefined: removeContainerNode`.

- [ ] **Step 3: Implement cleanup**

In `internal/orchestrator/container_node.go`, add:

```go
// removeContainerNode force-removes the service's container. Safe no-op if the
// container was never started. Called by the DAG cleanup on shutdown.
func removeContainerNode(n *Node) error {
	if n == nil || !n.containerStarted || n.Container == nil {
		return nil
	}
	_, rt, err := provider.Resolve(n.Container.ProviderName)
	if err != nil {
		return err
	}
	_, _ = rt.Exec("rm", "-f", containerNodeName(n.Name))
	return nil
}
```

In `internal/orchestrator/dag.go`, inside `cleanupFn` (after the `if n.crDeployed != nil { deleteCloudRunNode(n) }` block, around line 310):

```go
			if n.containerStarted {
				_ = removeContainerNode(n)
			}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/orchestrator/ -run TestRemoveContainerNode -v`
Expected: PASS.
Then full package + build:
Run: `go test ./internal/orchestrator/ ./cmd/ && go build ./...`
Expected: PASS, clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/container_node.go internal/orchestrator/dag.go internal/orchestrator/container_node_test.go
git commit -m "feat(orchestrator): remove runtime: container containers on shutdown"
```

---

## Manual verification (after all tasks)

Not unit-testable; do this once before considering the runtime done:

1. Add a `runtime: container` service to a scratch `devx.yaml` using a public image:
   ```yaml
   services:
     - name: web
       runtime: container
       container: { image: "nginx:1.27" }
       port: 8080
       healthcheck: { tcp: "localhost:8080" }
   ```
2. `devx up` → expect `🐳 Running web → container devx-svc-web (nginx:1.27)`, the container visible in `<runtime> ps`, and the TCP healthcheck passing.
3. Ctrl+C → expect `devx-svc-web` gone from `<runtime> ps -a` (cleanup ran).
4. Repeat with a `build:` block pointing at a local Dockerfile; confirm the image builds and runs.

## Notes for the engineer

- **Healthcheck is automatic.** Container nodes are not special-cased in the post-start health loop ([dag.go:407-420](../../internal/orchestrator/dag.go)); a container service with `healthcheck.http`/`healthcheck.tcp` is polled exactly like a host service. No code needed.
- **Logs are NOT in this plan.** Streaming container logs (inline + `~/.devx/logs/`) is Plan 2 (`implementation_plan_up_log_streaming.md`), which tails `devx-svc-<name>` — the name this plan establishes. Keep the `devx-svc-` prefix stable; Plan 2 depends on it.
- **Idempotent re-runs:** `startContainerNode` does `rm -f <name>` before `run` so a second `up` doesn't fail on a name clash.
