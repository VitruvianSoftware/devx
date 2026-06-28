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

// Package provider abstracts how a cluster node's K3s runtime is provisioned and
// managed, so the same orchestration works for a Lima VM on macOS and a native
// Linux host. The K3s install/join recipe is shared; providers differ in host
// preparation (VM vs. bare host) and command delivery.
package provider

import (
	"context"
	"time"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

// JoinOpts carries the K3s join recipe inputs shared by all providers.
type JoinOpts struct {
	NodeIP           string
	ServerURL        string
	Token            string
	Pool             string
	K3sVersion       string
	TLSSANs          []string
	DisableServiceLB bool
	UseTailscale     bool
	UseDocker        bool
	UseCilium        bool
}

// NodeProvider provisions and manages one node's K3s runtime.
type NodeProvider interface {
	// EnsureRuntime makes the node ready to run K3s and returns its cluster IP
	// (Lima: provision the VM; native: ensure deps/firewall and return the host IP).
	EnsureRuntime(ctx context.Context) (nodeIP string, err error)
	InstallServer(ctx context.Context, o JoinOpts) error
	InstallAgent(ctx context.Context, o JoinOpts) error
	GetToken(ctx context.Context) (string, error)
	IsInstalled(ctx context.Context) (bool, error)
	WaitForReady(ctx context.Context, timeout time.Duration) error
	NodeStatus(ctx context.Context) (string, error)
	Kubeconfig(ctx context.Context, serverIP string) (string, error)
	Drain(ctx context.Context, nodeName string) error
	DeleteNode(ctx context.Context, nodeName string) error
	Uninstall(ctx context.Context, role string) error
	Destroy(ctx context.Context) error
	Reconcile(ctx context.Context) error
}

// For returns the provider for a node, selected by its kind ("native" → direct
// K3s on a Linux host; anything else → Lima VM).
func For(node config.NodeConfig, cfg *config.Config) NodeProvider {
	if node.GetKind() == "native" {
		return NewNativeProvider(node, cfg)
	}
	return NewLimaProvider(node, cfg)
}

// ProviderSpec is the resolved, provider-facing view of a node + cluster config.
// Providers depend on this stable contract instead of reaching into
// config.NodeConfig / config.Config directly, so adding a config field ripples
// only to specFor (and the providers that actually need the new field) — not to
// every provider method.
type ProviderSpec struct {
	Host      string
	Role      string
	Pool      string
	Kind      string
	NodeIP    string
	VMName    string
	Tailscale config.TailscaleConfig
	Docker    config.DockerConfig
	Mounts    []config.MountConfig
}

// specFor builds the ProviderSpec for a node — the single adapter point between
// config.* and the providers.
func specFor(node config.NodeConfig, cfg *config.Config) ProviderSpec {
	return ProviderSpec{
		Host:      node.Host,
		Role:      node.Role,
		Pool:      node.Pool,
		Kind:      node.GetKind(),
		NodeIP:    node.NodeIP,
		VMName:    node.GetVMName(),
		Tailscale: cfg.Cluster.Tailscale,
		Docker:    cfg.Cluster.Docker,
		Mounts:    cfg.Cluster.Mounts,
	}
}
