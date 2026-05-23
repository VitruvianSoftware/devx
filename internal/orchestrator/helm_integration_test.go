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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const helmTestDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: devx-e2e-pause
spec:
  replicas: 1
  selector:
    matchLabels:
      app: devx-e2e-pause
  template:
    metadata:
      labels:
        app: devx-e2e-pause
    spec:
      containers:
        - name: pause
          image: registry.k8s.io/pause:3.9
`

// TestHelmDeploy_Integration exercises helmUpgradeInstall → helmUninstall against a
// REAL cluster with a minimal chart. Skipped unless DEVX_E2E_KUBECONFIG is set (and
// helm is installed), so `go test ./...` in CI never touches a cluster. helm runs
// host-side, so the chart can live in a normal TempDir.
//
//	DEVX_E2E_KUBECONFIG=$HOME/.kube/cluster.yaml DEVX_E2E_CONTEXT=default \
//	go test -run TestHelmDeploy_Integration -v ./internal/orchestrator/
func TestHelmDeploy_Integration(t *testing.T) {
	kubeconfig := os.Getenv("DEVX_E2E_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set DEVX_E2E_KUBECONFIG (e.g. ~/.kube/cluster.yaml) to run the helm deploy integration test")
	}
	kctx := os.Getenv("DEVX_E2E_CONTEXT")
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm CLI not installed")
	}

	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Chart.yaml"), "apiVersion: v2\nname: devx-e2e\nversion: 0.1.0\n")
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	mustWrite(t, filepath.Join(dir, "templates", "deployment.yaml"), helmTestDeployment)

	const ns = "devx-e2e-helm"
	const release = "devx-e2e"
	t.Cleanup(func() {
		_, _ = helmUninstall(context.Background(), kubeconfig, kctx, ns, release)
		_, _ = runKubectl(context.Background(),
			kubectlArgs(kubeconfig, kctx, "delete", "namespace", ns, "--ignore-not-found", "--wait=false")...)
	})

	if out, err := runKubectl(context.Background(),
		kubectlArgs(kubeconfig, kctx, "create", "namespace", ns)...); err != nil && !strings.Contains(out, "AlreadyExists") {
		t.Fatalf("create namespace: %v\n%s", err, out)
	}

	if err := helmUpgradeInstall(context.Background(), kubeconfig, kctx, ns, release, dir, nil); err != nil {
		t.Fatalf("helmUpgradeInstall: %v", err)
	}

	// helm should have rendered + applied the chart's Deployment.
	out, err := runKubectl(context.Background(),
		kubectlArgs(kubeconfig, kctx, "get", "deployment", "devx-e2e-pause", "-n", ns, "-o", "name")...)
	if err != nil {
		t.Fatalf("get deployment after helm install: %v\n%s", err, out)
	}
	if !strings.Contains(out, "devx-e2e-pause") {
		t.Fatalf("expected deployment devx-e2e-pause from helm chart, got %q", out)
	}

	if out, err := helmUninstall(context.Background(), kubeconfig, kctx, ns, release); err != nil {
		t.Fatalf("helmUninstall: %v\n%s", err, out)
	}
}
