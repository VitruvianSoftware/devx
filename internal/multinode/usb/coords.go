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

package usb

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
	"github.com/VitruvianSoftware/devx/internal/multinode/k3s"
	"github.com/VitruvianSoftware/devx/internal/multinode/lima"
	"github.com/VitruvianSoftware/devx/internal/multinode/util"
)

// Coordinates are the cluster-derived facts a USB node needs to join: where the
// API server is (on the LAN and/or the tailnet), the join token, and the k3s
// version to match.
type Coordinates struct {
	LANServerURL     string
	TailnetServerURL string
	Token            string
	K3sVersion       string
}

// ResolveCoordinates finds the first healthy server node, reads its join token
// and reachable API endpoints. It mirrors the healthy-server discovery in
// cluster.Join (kept separate to avoid changing Join's behavior; deduping the
// two is a noted follow-up).
//
// If cfg.Cluster.USB.AgentToken is set it is preferred over the server
// node-token, so operators can bake a least-privilege agent token that cannot
// add control-plane servers.
func ResolveCoordinates(ctx context.Context, cfg *config.Config) (Coordinates, error) {
	var c Coordinates
	c.K3sVersion = cfg.Cluster.K3sVersion

	for _, n := range cfg.ServerNodes() {
		runner := util.NewRunner(n)
		k3sMgr := k3s.NewManagerWithVM(runner, n.GetVMName())

		installed, _ := k3sMgr.IsInstalled(ctx)
		if !installed {
			continue
		}

		limaMgr := lima.NewManager(runner, n)
		ip, err := limaMgr.GetBridgedIP(ctx)
		if err != nil {
			slog.Debug("usb coords: skipping server without IP", "host", n.Host, "error", err)
			continue
		}

		token, err := k3sMgr.GetToken(ctx)
		if err != nil {
			slog.Debug("usb coords: skipping server without token", "host", n.Host, "error", err)
			continue
		}

		c.LANServerURL = fmt.Sprintf("https://%s:6443", ip)
		c.Token = token

		// Best-effort tailnet endpoint so nodes can join from other networks.
		if cfg.Cluster.Tailscale.Enabled {
			if tsIP, err := runner.LimaShellSudo(ctx, n.GetVMName(), "tailscale ip -4"); err == nil {
				if ip := strings.TrimSpace(tsIP); ip != "" {
					c.TailnetServerURL = fmt.Sprintf("https://%s:6443", ip)
				}
			}
		}
		break
	}

	if c.Token == "" {
		return c, fmt.Errorf("no healthy server node found to read the join token from")
	}

	// Explicit overrides from config.
	if t := cfg.Cluster.USB.AgentToken; t != "" {
		c.Token = t
	}
	if u := cfg.Cluster.USB.TailnetServerURL; u != "" {
		c.TailnetServerURL = u
	}
	return c, nil
}
