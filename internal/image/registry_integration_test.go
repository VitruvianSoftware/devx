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

package image

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestEnsureRegistry_Integration deploys the in-cluster registry to a REAL cluster
// and verifies it is idempotent and reachable. Skipped unless DEVX_E2E_KUBECONFIG
// is set, so `go test ./...` in CI never touches a cluster. Run against the local
// lima k3s cluster (NEVER a gke_*/prod context):
//
//	DEVX_E2E_KUBECONFIG=$HOME/.kube/cluster.yaml DEVX_E2E_CONTEXT=default \
//	go test -run TestEnsureRegistry_Integration -v ./internal/image/
func TestEnsureRegistry_Integration(t *testing.T) {
	kubeconfig := os.Getenv("DEVX_E2E_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set DEVX_E2E_KUBECONFIG (e.g. ~/.kube/cluster.yaml) to run the in-cluster registry integration test")
	}
	kctx := os.Getenv("DEVX_E2E_CONTEXT")

	// Tear down the registry namespace regardless of outcome.
	t.Cleanup(func() {
		_, _ = kubectl(context.Background(), kubeconfig, kctx,
			"delete", "namespace", RegistryNamespace, "--ignore-not-found", "--wait=false")
	})

	reg, err := EnsureRegistry(context.Background(), kubeconfig, kctx)
	if err != nil {
		t.Fatalf("EnsureRegistry: %v", err)
	}
	if reg.RefPrefix != "localhost:30500" {
		t.Errorf("RefPrefix = %q, want localhost:30500", reg.RefPrefix)
	}
	if !strings.HasSuffix(reg.PushAddr, ":30500") {
		t.Errorf("PushAddr = %q, want <node-ip>:30500", reg.PushAddr)
	}

	// Idempotency: a second call against an already-deployed registry must succeed.
	if _, err := EnsureRegistry(context.Background(), kubeconfig, kctx); err != nil {
		t.Fatalf("EnsureRegistry (second call — idempotency): %v", err)
	}

	// The registry's v2 catalog should be reachable from the host via PushAddr.
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://" + reg.PushAddr + "/v2/_catalog")
	if err != nil {
		t.Fatalf("GET registry catalog at %s: %v", reg.PushAddr, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "repositories") {
		t.Errorf("registry catalog response = %q, want a repositories list", string(body))
	}
}
