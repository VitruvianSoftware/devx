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

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/image"
	"github.com/VitruvianSoftware/devx/internal/logs"
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

// containerImageRef computes the local image tag for a built container service.
func containerImageRef(service string, spec *image.Spec) string {
	tag := spec.Tag
	if tag == "" {
		tag = "dev"
	}
	return fmt.Sprintf("%s-%s:%s", containerNodeBase, service, tag)
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
	streamContainerLogs(ctx, rt, n)
	fmt.Printf("  ✅ %s running\n", n.Name)
	return nil
}

// containerLogsTailArgs builds the `<runtime> logs -f` args for a container.
func containerLogsTailArgs(name string) []string {
	return []string{"logs", "--tail", "50", "-f", name}
}

// streamContainerLogs tails the container's logs into the service sink (inline +
// file) when LogMode is on. Runs until ctx is cancelled. No-op for LogOff.
func streamContainerLogs(ctx context.Context, rt provider.ContainerRuntime, n *Node) {
	if n.LogMode == LogOff {
		return
	}
	w, closeFn, err := logs.BuildSink(n.Name, sinkMode(n.LogMode), os.Stdout, logs.ColorEnabled(), nil)
	if err != nil {
		return
	}
	n.logCloser = closeFn
	logCtx, cancel := context.WithCancel(ctx)
	n.logWatchCancel = cancel
	cmd := rt.CommandContext(logCtx, containerLogsTailArgs(containerNodeName(n.Name))...)
	cmd.Stdout = w
	cmd.Stderr = w
	go func() { _ = cmd.Run() }() // ends when logCtx is cancelled on shutdown
}

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
