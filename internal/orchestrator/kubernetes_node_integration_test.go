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
	"os"
	"path/filepath"
	"testing"
)

// pauseDeploymentYAML is a trivial, fast-to-pull workload that becomes Available
// as soon as its single pod is Running (pause has no readiness probe, so Ready =
// Running). This keeps the integration test independent of any app image.
const pauseDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: smoke-pause
spec:
  replicas: 1
  selector:
    matchLabels:
      app: smoke-pause
  template:
    metadata:
      labels:
        app: smoke-pause
    spec:
      containers:
        - name: pause
          image: registry.k8s.io/pause:3.9
          imagePullPolicy: IfNotPresent
`

const kustomizationYAML = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
`

// TestStartKubernetesNode_Integration exercises the full apply → readiness-gate →
// delete lifecycle against a REAL cluster. It is skipped unless DEVX_E2E_KUBECONFIG
// points at a kubeconfig, so `go test ./...` in CI never touches a cluster. Run it
// against the local lima k3s cluster (NEVER a gke_*/prod context):
//
//	DEVX_E2E_KUBECONFIG=$HOME/.kube/cluster.yaml \
//	DEVX_E2E_CONTEXT=default \
//	go test -run TestStartKubernetesNode_Integration -v ./internal/orchestrator/
func TestStartKubernetesNode_Integration(t *testing.T) {
	kubeconfig := os.Getenv("DEVX_E2E_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set DEVX_E2E_KUBECONFIG (e.g. ~/.kube/cluster.yaml) to run the kubernetes deploy integration test")
	}
	kctx := os.Getenv("DEVX_E2E_CONTEXT")

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "deployment.yaml"), pauseDeploymentYAML)
	mustWrite(t, filepath.Join(dir, "kustomization.yaml"), kustomizationYAML)

	const ns = "devx-e2e-smoke"
	node := &Node{
		Name:    "smoke",
		Runtime: RuntimeKubernetes,
		Kube: &KubeNodeConfig{
			Manifests:  dir,
			Renderer:   "kustomize",
			Namespace:  ns,
			Context:    kctx,
			Kubeconfig: kubeconfig,
		},
	}

	// Guarantee the namespace is torn down even if an assertion fails midway.
	t.Cleanup(func() {
		_, _ = runKubectl(context.Background(),
			kubectlArgs(kubeconfig, kctx, "delete", "namespace", ns, "--ignore-not-found", "--wait=false")...)
	})

	if err := startKubernetesNode(context.Background(), node); err != nil {
		t.Fatalf("startKubernetesNode: %v", err)
	}
	if node.kubeApplied == nil {
		t.Fatal("expected kubeApplied to be recorded after a successful apply")
	}

	// startKubernetesNode only returns once the deployment is Available; confirm it.
	const jsonpath = "jsonpath={.status.conditions[?(@.type==\"Available\")].status}"
	out, err := runKubectl(context.Background(),
		kubectlArgs(kubeconfig, kctx, "get", "deployment", "-n", ns, "smoke-pause", "-o", jsonpath)...)
	if err != nil {
		t.Fatalf("get deployment: %v\n%s", err, out)
	}
	if out != "True" {
		t.Fatalf("expected deployment Available=True, got %q", out)
	}

	// deleteKubernetesNode should remove what we applied.
	deleteKubernetesNode(node)
	out, err = runKubectl(context.Background(),
		kubectlArgs(kubeconfig, kctx, "get", "deployment", "-n", ns, "smoke-pause", "--ignore-not-found")...)
	if err != nil {
		t.Fatalf("get after delete: %v\n%s", err, out)
	}
	if out != "" {
		t.Errorf("expected deployment to be deleted, kubectl still reports:\n%s", out)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
