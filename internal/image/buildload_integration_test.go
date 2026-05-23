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
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBuildAndLoad_Integration exercises the full build→push path through the devx
// code against a REAL cluster: it ensures the in-cluster registry, builds a trivial
// image with the configured provider, pushes it, and verifies it lands in the
// registry catalog. Skipped unless DEVX_E2E_KUBECONFIG is set. The build context is
// created under the package directory because the lima builder VM mounts the repo
// path (a /var/folders TempDir would not be visible inside the VM).
//
//	DEVX_E2E_KUBECONFIG=$HOME/.kube/cluster.yaml DEVX_E2E_CONTEXT=default \
//	DEVX_E2E_PROVIDER=lima go test -run TestBuildAndLoad_Integration -v ./internal/image/
func TestBuildAndLoad_Integration(t *testing.T) {
	kubeconfig := os.Getenv("DEVX_E2E_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set DEVX_E2E_KUBECONFIG (e.g. ~/.kube/cluster.yaml) to run the build+load integration test")
	}
	kctx := os.Getenv("DEVX_E2E_CONTEXT")
	prov := os.Getenv("DEVX_E2E_PROVIDER")
	if prov == "" {
		prov = "lima"
	}

	ctxDir, err := os.MkdirTemp(".", "devx-e2e-ctx-")
	if err != nil {
		t.Fatalf("temp context dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(ctxDir) }()
	absCtx, err := filepath.Abs(ctxDir)
	if err != nil {
		t.Fatalf("abs context: %v", err)
	}
	if err := os.WriteFile(filepath.Join(absCtx, "Dockerfile"),
		[]byte("FROM registry.k8s.io/pause:3.9\n"), 0o600); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}

	t.Cleanup(func() {
		_, _ = kubectl(context.Background(), kubeconfig, kctx,
			"delete", "namespace", RegistryNamespace, "--ignore-not-found", "--wait=false")
	})

	const imgName = "devx-e2e-buildload"
	reg, err := EnsureRegistry(context.Background(), kubeconfig, kctx)
	if err != nil {
		t.Fatalf("EnsureRegistry: %v", err)
	}

	if err := BuildAndLoad(context.Background(), prov, "", kubeconfig, kctx,
		[]Spec{{Name: imgName, Context: absCtx}}); err != nil {
		t.Fatalf("BuildAndLoad (provider %q): %v", prov, err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://" + reg.PushAddr + "/v2/_catalog")
	if err != nil {
		t.Fatalf("GET registry catalog: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), imgName) {
		t.Fatalf("registry catalog %q does not contain pushed image %q", string(body), imgName)
	}
}
