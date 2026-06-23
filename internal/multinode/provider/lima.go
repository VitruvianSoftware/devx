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

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/k3s"
	"github.com/VitruvianSoftware/devx/internal/multinode/lima"
	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// LimaProvider provisions a node as a Lima VM on a (macOS) host — the original
// behavior, now behind the NodeProvider interface. It is a thin wrapper over the
// existing lima.Manager and k3s.Manager.
type LimaProvider struct {
	node config.NodeConfig
	cfg  *config.Config
	run  *remote.Runner
	lima *lima.Manager
	k3s  *k3s.Manager
}

// NewLimaProvider builds a LimaProvider for the node.
func NewLimaProvider(node config.NodeConfig, cfg *config.Config) *LimaProvider {
	run := util.NewRunner(node)
	return &LimaProvider{
		node: node, cfg: cfg, run: run,
		lima: lima.NewManager(run, node),
		k3s:  k3s.NewManagerWithVM(run, node.GetVMName()),
	}
}

func (p *LimaProvider) EnsureRuntime(ctx context.Context) (string, error) {
	if err := p.lima.Provision(ctx, p.cfg.Cluster.Docker, p.cfg.Cluster.Mounts); err != nil {
		return "", err
	}
	ip, err := p.lima.GetBridgedIP(ctx)
	if err != nil {
		return "", err
	}
	if p.cfg.Cluster.Tailscale.Enabled && p.cfg.Cluster.Tailscale.AuthKey != "" {
		tsIP, err := p.k3s.InstallTailscale(ctx, p.cfg.Cluster.Tailscale.AuthKey)
		if err != nil {
			return "", err
		}
		ip = tsIP
	}
	return ip, nil
}

func (p *LimaProvider) InstallServer(ctx context.Context, o JoinOpts) error {
	return p.k3s.JoinServer(ctx, o.NodeIP, o.ServerURL, o.Token, o.Pool, o.K3sVersion, o.TLSSANs, o.DisableServiceLB, o.UseTailscale, o.UseDocker, o.UseCilium)
}

func (p *LimaProvider) InstallAgent(ctx context.Context, o JoinOpts) error {
	return p.k3s.JoinAgent(ctx, o.NodeIP, o.ServerURL, o.Token, o.Pool, o.K3sVersion, o.UseTailscale, o.UseDocker, o.UseCilium)
}

func (p *LimaProvider) GetToken(ctx context.Context) (string, error) { return p.k3s.GetToken(ctx) }
func (p *LimaProvider) IsInstalled(ctx context.Context) (bool, error) {
	return p.k3s.IsInstalled(ctx)
}
func (p *LimaProvider) WaitForReady(ctx context.Context, d time.Duration) error {
	return p.k3s.WaitForReady(ctx, d)
}
func (p *LimaProvider) NodeStatus(ctx context.Context) (string, error) {
	return p.k3s.GetNodeStatus(ctx)
}
func (p *LimaProvider) Kubeconfig(ctx context.Context, ip string) (string, error) {
	return p.k3s.GetKubeconfig(ctx, ip)
}
func (p *LimaProvider) Drain(ctx context.Context, n string) error { return p.k3s.DrainNode(ctx, n) }
func (p *LimaProvider) DeleteNode(ctx context.Context, n string) error {
	return p.k3s.DeleteNode(ctx, n)
}
func (p *LimaProvider) Uninstall(ctx context.Context, role string) error {
	return p.k3s.Uninstall(ctx, role)
}
func (p *LimaProvider) Destroy(ctx context.Context) error {
	_ = p.k3s.Uninstall(ctx, p.node.Role)
	return p.lima.Destroy(ctx)
}
func (p *LimaProvider) Reconcile(ctx context.Context) error {
	_, err := p.run.LimaShellSudo(ctx, p.node.GetVMName(),
		fmt.Sprintf("apt-get update -qq && apt-get install -y -qq %s", lima.NodePackages))
	return err
}
