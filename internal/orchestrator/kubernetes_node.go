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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/bridge"
)

// kubectlArgs builds a fresh kubectl arg slice with --kubeconfig/--context flags
// plus the given subcommand args. Returns a NEW slice each call to avoid
// append-aliasing across multiple invocations.
func kubectlArgs(kubeconfig, kctx string, extra ...string) []string {
	args := []string{"--kubeconfig", kubeconfig}
	if kctx != "" {
		args = append(args, "--context", kctx)
	}
	return append(args, extra...)
}

func runKubectl(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "kubectl", args...).CombinedOutput()
	return string(out), err
}

// resolveManifestsPath resolves a (possibly relative) manifests path against the
// node's working dir (set for multirepo includes) or the current directory.
func resolveManifestsPath(n *Node, manifests string) string {
	if filepath.IsAbs(manifests) {
		return manifests
	}
	base := n.Dir
	if base == "" {
		base, _ = os.Getwd()
	}
	return filepath.Join(base, manifests)
}

// applyFlagForRenderer maps a kubernetes renderer to the kubectl path flag used by
// both `apply` and `delete`: kustomize → "-k" (kubectl's built-in kustomize), raw →
// "-f". helm is recognized but not yet implemented; any other value is rejected. It
// is a pure function so the renderer can be validated up front, before any cluster
// mutation, and so apply/delete share one source of truth for the mapping.
func applyFlagForRenderer(renderer string) (string, error) {
	switch renderer {
	case "kustomize":
		return "-k", nil
	case "raw":
		return "-f", nil
	case "helm":
		return "", fmt.Errorf("kubernetes.renderer %q is not implemented yet (use kustomize or raw)", renderer)
	default:
		return "", fmt.Errorf("unknown kubernetes.renderer %q (want kustomize, raw, or helm)", renderer)
	}
}

// startKubernetesNode renders a runtime: kubernetes service's manifests and applies
// them to the target cluster via kubectl (skaffold-class deploy). devx shells out to
// kubectl — kustomize is built in (`apply -k`) — consistent with the bridge / db /
// cloudflared subprocess pattern. It blocks until the namespace's Deployments are
// Available, so the DAG's generic HTTP/TCP healthcheck is skipped for k8s nodes.
func startKubernetesNode(ctx context.Context, n *Node) error {
	k := n.Kube
	if k == nil || k.Manifests == "" {
		return fmt.Errorf("service %q has runtime: kubernetes but no kubernetes.manifests is set", n.Name)
	}

	if _, err := bridge.ValidateKubectl(); err != nil {
		return err
	}
	kubeconfig, err := bridge.ResolveKubeconfig(k.Kubeconfig)
	if err != nil {
		return err
	}
	if err := bridge.ValidateContext(kubeconfig, k.Context); err != nil {
		return err
	}

	ns := k.Namespace
	if ns == "" {
		ns = "default"
	}
	renderer := k.Renderer
	if renderer == "" {
		renderer = "kustomize"
	}
	manifests := resolveManifestsPath(n, k.Manifests)

	// Validate the renderer up front so a bad config fails fast — before we touch
	// the cluster (e.g. create a namespace we'd then orphan).
	flag, err := applyFlagForRenderer(renderer)
	if err != nil {
		return fmt.Errorf("service %q: %w", n.Name, err)
	}

	// Idempotently ensure the target namespace exists.
	if out, err := runKubectl(ctx, kubectlArgs(kubeconfig, k.Context, "create", "namespace", ns)...); err != nil &&
		!strings.Contains(out, "AlreadyExists") {
		return fmt.Errorf("ensuring namespace %q: %w\n%s", ns, err, strings.TrimSpace(out))
	}

	applyArgs := kubectlArgs(kubeconfig, k.Context, "apply", "-n", ns, flag, manifests)

	fmt.Printf("  ☸️  Deploying %s (%s) %s → ns/%s\n", n.Name, renderer, manifests, ns)
	if out, err := runKubectl(ctx, applyArgs...); err != nil {
		return fmt.Errorf("kubectl apply for %q failed: %w\n%s", n.Name, err, strings.TrimSpace(out))
	}

	// Readiness gate: wait for the namespace's Deployments to become Available
	// (in-cluster, no port-forward needed).
	fmt.Printf("  ⏳ Waiting for %s deployments to become Available...\n", n.Name)
	waitArgs := kubectlArgs(kubeconfig, k.Context, "wait", "-n", ns,
		"--for=condition=Available", "deployment", "--all", "--timeout=120s")
	if out, err := runKubectl(ctx, waitArgs...); err != nil {
		return fmt.Errorf("deployments for %q did not become Available: %w\n%s", n.Name, err, strings.TrimSpace(out))
	}

	// Record the resolved deploy (abs manifests path) so cleanup can delete it.
	n.kubeApplied = &KubeNodeConfig{
		Manifests:  manifests,
		Renderer:   renderer,
		Namespace:  ns,
		Context:    k.Context,
		Kubeconfig: kubeconfig,
	}
	fmt.Printf("  ✅ %s deployed\n", n.Name)
	return nil
}

// deleteKubernetesNode removes what startKubernetesNode applied (best-effort, on `devx down`).
func deleteKubernetesNode(n *Node) {
	k := n.kubeApplied
	if k == nil {
		return
	}
	// The renderer was validated at apply time (kubeApplied is only set after a
	// successful apply), so this won't error in practice; default to "-k" if it does.
	flag, err := applyFlagForRenderer(k.Renderer)
	if err != nil {
		flag = "-k"
	}
	args := kubectlArgs(k.Kubeconfig, k.Context, "delete", "-n", k.Namespace, flag, k.Manifests, "--ignore-not-found")
	if out, err := runKubectl(context.Background(), args...); err != nil {
		fmt.Printf("  ⚠️  cleanup: kubectl delete for %s failed: %s\n", n.Name, strings.TrimSpace(out))
	}
}
