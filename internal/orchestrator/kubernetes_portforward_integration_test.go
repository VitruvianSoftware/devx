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

const pfTestManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: devx-e2e-pf
spec:
  replicas: 1
  selector:
    matchLabels:
      app: devx-e2e-pf
  template:
    metadata:
      labels:
        app: devx-e2e-pf
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: devx-e2e-pf
spec:
  selector:
    app: devx-e2e-pf
  ports:
    - port: 80
      targetPort: 80
`

// TestPortForward_Integration deploys nginx + a Service to a REAL cluster and runs
// startPortForwards, asserting devx discovers the Service and launches a forward to
// a conflict-resolved local port. Skipped unless DEVX_E2E_KUBECONFIG is set.
//
// It deliberately does NOT assert traffic reachability: `kubectl port-forward`
// requires `socat` on the cluster nodes to carry traffic, which is a cluster
// prerequisite outside devx's control (the lima dev cluster's nodes lack it). devx's
// responsibility — discovering the Service and launching the correct forward — is
// what's verified here; parsing is covered by TestParseServiceForwards.
//
//	DEVX_E2E_KUBECONFIG=$HOME/.kube/cluster.yaml DEVX_E2E_CONTEXT=default \
//	go test -run TestPortForward_Integration -v ./internal/orchestrator/
func TestPortForward_Integration(t *testing.T) {
	kubeconfig := os.Getenv("DEVX_E2E_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set DEVX_E2E_KUBECONFIG (e.g. ~/.kube/cluster.yaml) to run the port-forward integration test")
	}
	kctx := os.Getenv("DEVX_E2E_CONTEXT")
	bg := context.Background()
	const ns = "devx-e2e-pf"

	t.Cleanup(func() {
		_, _ = runKubectl(bg, kubectlArgs(kubeconfig, kctx, "delete", "namespace", ns, "--ignore-not-found", "--wait=false")...)
	})

	dir := t.TempDir()
	manifest := filepath.Join(dir, "pf.yaml")
	mustWrite(t, manifest, pfTestManifest)
	if out, err := runKubectl(bg, kubectlArgs(kubeconfig, kctx, "create", "namespace", ns)...); err != nil && !strings.Contains(out, "AlreadyExists") {
		t.Fatalf("create namespace: %v\n%s", err, out)
	}
	if out, err := runKubectl(bg, kubectlArgs(kubeconfig, kctx, "apply", "-n", ns, "-f", manifest)...); err != nil {
		t.Fatalf("apply nginx: %v\n%s", err, out)
	}
	if out, err := runKubectl(bg, kubectlArgs(kubeconfig, kctx, "wait", "-n", ns,
		"--for=condition=Available", "deployment/devx-e2e-pf", "--timeout=120s")...); err != nil {
		t.Fatalf("wait for nginx: %v\n%s", err, out)
	}

	forwards, cancel, err := startPortForwards(bg, kubeconfig, kctx, ns)
	if err != nil {
		t.Fatalf("startPortForwards: %v", err)
	}
	defer cancel()

	found := false
	for _, f := range forwards {
		if f.Service == "devx-e2e-pf" && f.Port == 80 && f.Local > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("nginx Service forward not discovered/launched, got %v", forwards)
	}
}
