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

package cluster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/provider"
)

// fakeNodeProvider records Reconcile calls; every other method is a no-op so it
// satisfies provider.NodeProvider for dispatch tests.
type fakeNodeProvider struct{ onReconcile func() }

func (f *fakeNodeProvider) EnsureRuntime(context.Context) (string, error)        { return "", nil }
func (f *fakeNodeProvider) InstallServer(context.Context, provider.JoinOpts) error { return nil }
func (f *fakeNodeProvider) InstallAgent(context.Context, provider.JoinOpts) error  { return nil }
func (f *fakeNodeProvider) GetToken(context.Context) (string, error)             { return "", nil }
func (f *fakeNodeProvider) IsInstalled(context.Context) (bool, error)            { return true, nil }
func (f *fakeNodeProvider) WaitForReady(context.Context, time.Duration) error    { return nil }
func (f *fakeNodeProvider) NodeStatus(context.Context) (string, error)           { return "", nil }
func (f *fakeNodeProvider) Kubeconfig(context.Context, string) (string, error)   { return "", nil }
func (f *fakeNodeProvider) Drain(context.Context, string) error                  { return nil }
func (f *fakeNodeProvider) DeleteNode(context.Context, string) error             { return nil }
func (f *fakeNodeProvider) Uninstall(context.Context, string) error              { return nil }
func (f *fakeNodeProvider) Destroy(context.Context) error                        { return nil }
func (f *fakeNodeProvider) Reconcile(context.Context) error {
	if f.onReconcile != nil {
		f.onReconcile()
	}
	return nil
}

// Regression for #372: cluster.Reconcile must converge EVERY node — including
// native ones — through its NodeProvider (NativeProvider.Reconcile runs
// ensureHostPrereqs with the right package manager), not a Lima-only `apt-get`
// path that errors on native hosts (no Lima VM) and uses the wrong package
// manager for Fedora.
func TestReconcileNodes_DispatchesProviderForEveryNode(t *testing.T) {
	orig := providerFor
	t.Cleanup(func() { providerFor = orig })

	var mu sync.Mutex
	seen := map[string]bool{}
	providerFor = func(node config.NodeConfig, _ *config.Config) provider.NodeProvider {
		return &fakeNodeProvider{onReconcile: func() {
			mu.Lock()
			seen[node.Host] = true
			mu.Unlock()
		}}
	}

	cfg := &config.Config{Nodes: []config.NodeConfig{
		{Host: "fedora", Role: "server", Kind: "native"},
		{Host: "nuc9", Role: "agent", Kind: "native"},
		{Host: "mbp", Role: "agent"}, // lima (default kind)
	}}

	if err := reconcileNodes(context.Background(), cfg); err != nil {
		t.Fatalf("reconcileNodes: %v", err)
	}
	for _, h := range []string{"fedora", "nuc9", "mbp"} {
		if !seen[h] {
			t.Errorf("node %q was not reconciled via its provider", h)
		}
	}
}
