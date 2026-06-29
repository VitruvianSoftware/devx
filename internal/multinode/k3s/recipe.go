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

package k3s

import (
	"fmt"
	"strings"
)

// This file renders the get.k3s.io install recipes as pure, unit-testable
// functions, so the exact flags a node is installed with can be asserted without
// a live host (the InitCluster/JoinServer/JoinAgent methods build the recipe here
// and only execute it).

// k3sVersionEnv renders the INSTALL_K3S_VERSION env prefix (empty = latest).
func k3sVersionEnv(version string) string {
	if version == "" {
		return ""
	}
	return fmt.Sprintf("INSTALL_K3S_VERSION=%q ", version)
}

// tlsSANFlags renders one --tls-san flag per SAN (server nodes only).
func tlsSANFlags(sans []string) string {
	var b strings.Builder
	for _, san := range sans {
		fmt.Fprintf(&b, " --tls-san=%s", san)
	}
	return b.String()
}

// flannelIface returns the --flannel-iface flag for the chosen network backbone.
func flannelIface(useTailscale bool) string {
	if useTailscale {
		return " --flannel-iface=tailscale0"
	}
	return " --flannel-iface=lima0"
}

// serverExtraArgs renders the server-only install flags: ServiceLB toggle, the
// flannel interface, the Docker runtime, and the Cilium CNI-takeover flags.
func serverExtraArgs(disableServiceLB, useTailscale, useDocker, useCilium bool) string {
	var b strings.Builder
	if disableServiceLB {
		b.WriteString(" --disable servicelb")
	}
	b.WriteString(flannelIface(useTailscale))
	if useDocker {
		b.WriteString(" --docker")
	}
	if useCilium {
		// k3s embeds Flannel + kube-proxy + the netpol controller; Cilium replaces all three.
		b.WriteString(" --flannel-backend=none --disable-kube-proxy --disable-network-policy")
		// Platform (Cilium/ArgoCD) clusters replace k3s's single-replica bundled
		// metrics-server with the HA metrics-server gitops appset (2 replicas).
		// Disable the built-in so the two don't fight the same kube-system Deployment
		// and the single v1beta1.metrics.k8s.io APIService. Non-platform k3s (lima,
		// etc.) keeps the bundled metrics-server. (useCilium is the platform signal.)
		b.WriteString(" --disable metrics-server")
	}
	return b.String()
}

// agentExtraArgs renders the agent install flags. Agents take NO Cilium disable
// flags — --flannel-backend=none / --disable-kube-proxy / --disable-network-policy
// are SERVER-only (the agent CLI rejects them); agents inherit "none" from the
// servers' cluster config.
func agentExtraArgs(useTailscale, useDocker bool) string {
	var b strings.Builder
	b.WriteString(flannelIface(useTailscale))
	if useDocker {
		b.WriteString(" --docker")
	}
	return b.String()
}

// buildServerInitScript renders the recipe for the first control-plane node
// (--cluster-init).
func buildServerInitScript(host, nodeIP, pool, version string, tlsSANs []string, disableServiceLB, useTailscale, useDocker, useCilium bool) string {
	return fmt.Sprintf(
		`curl -sfL https://get.k3s.io | %sINSTALL_K3S_EXEC="server" sh -s - --cluster-init --node-name=%s --node-ip=%s --advertise-address=%s%s%s --node-label=pool=%s`,
		k3sVersionEnv(version), host, nodeIP, nodeIP, tlsSANFlags(tlsSANs), serverExtraArgs(disableServiceLB, useTailscale, useDocker, useCilium), pool,
	)
}

// buildServerJoinScript renders the recipe for a server joining an existing
// cluster — installed but not started (SKIP_START), so the join can retry on its
// own while etcd stabilizes.
func buildServerJoinScript(host, nodeIP, serverURL, token, pool, version string, tlsSANs []string, disableServiceLB, useTailscale, useDocker, useCilium bool) string {
	return fmt.Sprintf(
		`curl -sfL https://get.k3s.io | %sINSTALL_K3S_SKIP_START=true K3S_TOKEN=%q INSTALL_K3S_EXEC="server" sh -s - --server=%s --node-name=%s --node-ip=%s --advertise-address=%s%s%s --node-label=pool=%s`,
		k3sVersionEnv(version), token, serverURL, host, nodeIP, nodeIP, tlsSANFlags(tlsSANs), serverExtraArgs(disableServiceLB, useTailscale, useDocker, useCilium), pool,
	)
}

// buildAgentJoinScript renders the recipe for a worker node joining the cluster.
func buildAgentJoinScript(host, nodeIP, serverURL, token, pool, version string, useTailscale, useDocker bool) string {
	return fmt.Sprintf(
		`curl -sfL https://get.k3s.io | %sK3S_TOKEN=%q K3S_URL=%q sh -s - agent --node-name=%s --node-ip=%s%s --node-label=pool=%s`,
		k3sVersionEnv(version), token, serverURL, host, nodeIP, agentExtraArgs(useTailscale, useDocker), pool,
	)
}
