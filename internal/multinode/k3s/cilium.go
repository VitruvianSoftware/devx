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
	"context"
	"fmt"
	"strings"
)

// Cilium native routing over tailscale needs every node to advertise its pod
// CIDR as a tailnet subnet route (so pod traffic is tailnet-reachable across
// nodes), and the stable LAN nodes to also advertise the MetalLB LB range. These
// helpers + Manager methods bake those node-level settings into provisioning;
// they do NOT install the Cilium CNI itself (that lives in GitOps).

// ReadNodePodCIDR reads a node's assigned pod CIDR from the cluster via kubectl
// jsonpath. It must run on a control-plane node that has kubectl (the Manager's
// exec), and the node must already have joined so its podCIDR is assigned.
func (m *Manager) ReadNodePodCIDR(ctx context.Context, nodeName string) (string, error) {
	out, err := m.sudo(ctx, fmt.Sprintf(
		"k3s kubectl get node %s -o jsonpath='{.spec.podCIDR}'", nodeName))
	if err != nil {
		return "", fmt.Errorf("[%s] reading podCIDR for node %s: %w", m.runner.Host, nodeName, err)
	}
	return strings.TrimSpace(out), nil
}

// advertiseRoutesCmd builds the `tailscale set` command that advertises the
// node's pod CIDR (plus any extra routes, e.g. the LB range) as tailnet subnet
// routes, while keeping accept-routes on so the node also imports peers' routes.
func advertiseRoutesCmd(podCIDR string, extraRoutes []string) string {
	routes := append([]string{podCIDR}, extraRoutes...)
	return fmt.Sprintf("tailscale set --advertise-routes=%s --accept-routes=true",
		strings.Join(routes, ","))
}

// AdvertiseNodeRoutes runs the tailscale route-advertisement on the node itself
// (the Manager's exec is bound to that node). podCIDR is the node's pod CIDR;
// extraRoutes carries any additional subnet routes (the LB range on stable nodes).
func (m *Manager) AdvertiseNodeRoutes(ctx context.Context, podCIDR string, extraRoutes []string) error {
	if podCIDR == "" {
		return fmt.Errorf("[%s] cannot advertise routes: empty podCIDR", m.runner.Host)
	}
	if _, err := m.sudo(ctx, advertiseRoutesCmd(podCIDR, extraRoutes)); err != nil {
		return fmt.Errorf("[%s] advertising tailscale routes: %w", m.runner.Host, err)
	}
	return nil
}

// LBRangeCIDR derives the /24 subnet carrying a MetalLB IP range so the LB IPs
// can be advertised as a single tailnet subnet route. It accepts the range in
// any of the forms MetalLB allows: an explicit CIDR ("10.44.86.0/24"), a dashed
// range with full endpoints ("10.44.86.210-10.44.86.215") or a bare-suffix
// endpoint ("10.44.86.210-215"), or a single IP ("10.44.86.210"). The host octet
// is always zeroed to a /24. An empty input yields an empty string.
func LBRangeCIDR(ipRange string) string {
	ipRange = strings.TrimSpace(ipRange)
	if ipRange == "" {
		return ""
	}
	// If already a CIDR, normalise to the /24 base.
	if i := strings.IndexByte(ipRange, '/'); i >= 0 {
		ipRange = ipRange[:i]
	}
	// Take the start of a dashed range (the bit before the dash).
	if i := strings.IndexByte(ipRange, '-'); i >= 0 {
		ipRange = ipRange[:i]
	}
	octets := strings.Split(ipRange, ".")
	if len(octets) != 4 {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s.0/24", octets[0], octets[1], octets[2])
}
