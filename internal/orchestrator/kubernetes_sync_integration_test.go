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
	"strings"
	"testing"
)

const syncTestDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: devx-e2e-sync
spec:
  replicas: 1
  selector:
    matchLabels:
      app: devx-e2e-sync
  template:
    metadata:
      labels:
        app: devx-e2e-sync
    spec:
      containers:
        - name: app
          image: busybox:1.36
          command: ["sleep", "3600"]
`

// TestPodSync_Integration exercises the live-reload building blocks (discoverPod +
// kubectlCp) against a REAL cluster: it deploys a busybox pod (which has `tar`, so
// `kubectl cp` works), copies a local file in, and verifies it landed. Skipped
// unless DEVX_E2E_KUBECONFIG is set.
//
//	DEVX_E2E_KUBECONFIG=$HOME/.kube/cluster.yaml DEVX_E2E_CONTEXT=default \
//	go test -run TestPodSync_Integration -v ./internal/orchestrator/
func TestPodSync_Integration(t *testing.T) {
	kubeconfig := os.Getenv("DEVX_E2E_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set DEVX_E2E_KUBECONFIG (e.g. ~/.kube/cluster.yaml) to run the pod-sync integration test")
	}
	kctx := os.Getenv("DEVX_E2E_CONTEXT")
	bg := context.Background()
	const ns = "devx-e2e-sync"

	t.Cleanup(func() {
		_, _ = runKubectl(bg, kubectlArgs(kubeconfig, kctx, "delete", "namespace", ns, "--ignore-not-found", "--wait=false")...)
	})

	dir := t.TempDir()
	manifest := filepath.Join(dir, "deploy.yaml")
	mustWrite(t, manifest, syncTestDeployment)

	if out, err := runKubectl(bg, kubectlArgs(kubeconfig, kctx, "create", "namespace", ns)...); err != nil && !strings.Contains(out, "AlreadyExists") {
		t.Fatalf("create namespace: %v\n%s", err, out)
	}
	if out, err := runKubectl(bg, kubectlArgs(kubeconfig, kctx, "apply", "-n", ns, "-f", manifest)...); err != nil {
		t.Fatalf("apply busybox deployment: %v\n%s", err, out)
	}
	if out, err := runKubectl(bg, kubectlArgs(kubeconfig, kctx, "wait", "-n", ns,
		"--for=condition=Available", "deployment/devx-e2e-sync", "--timeout=120s")...); err != nil {
		t.Fatalf("wait for busybox: %v\n%s", err, out)
	}

	// discoverPod should find the running busybox pod.
	pod, err := discoverPod(bg, kubeconfig, kctx, ns, "app=devx-e2e-sync")
	if err != nil {
		t.Fatalf("discoverPod: %v", err)
	}

	// kubectlCp a local file into the pod, then read it back to confirm.
	local := filepath.Join(dir, "synced.txt")
	mustWrite(t, local, "hello-from-devx-live-reload")
	if err := kubectlCp(bg, kubeconfig, kctx, ns, pod, "", local, "/tmp/synced.txt"); err != nil {
		t.Fatalf("kubectlCp: %v", err)
	}
	out, err := runKubectl(bg, kubectlArgs(kubeconfig, kctx, "exec", "-n", ns, pod, "--", "cat", "/tmp/synced.txt")...)
	if err != nil {
		t.Fatalf("exec cat: %v\n%s", err, out)
	}
	if !strings.Contains(out, "hello-from-devx-live-reload") {
		t.Fatalf("synced file content not found in pod, got %q", out)
	}
}
