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
	"fmt"
	"sort"
	"strings"
)

// nonHTTPPorts are well-known ports whose protocol isn't HTTP. The access
// summary shows these as a bare host:port rather than a misleading http:// URL
// (e.g. a forwarded Postgres should read "localhost:5432", not a clickable URL).
var nonHTTPPorts = map[int]bool{
	5432:  true, // postgres
	3306:  true, // mysql / mariadb
	6379:  true, // redis
	27017: true, // mongodb
	5672:  true, // amqp / rabbitmq
	9092:  true, // kafka
}

// accessURL renders the address a developer connects to for a forwarded port:
// host:port uses the local (forwarded) port, while the scheme is decided by the
// service port — the stable protocol indicator. So a non-HTTP service (e.g.
// Postgres) whose local port was conflict-shifted still reads as a bare
// host:port, never a misleading http:// URL.
func accessURL(localPort, svcPort int) string {
	if nonHTTPPorts[svcPort] {
		return fmt.Sprintf("localhost:%d", localPort)
	}
	return fmt.Sprintf("http://localhost:%d", localPort)
}

// formatAccessSummary renders the consolidated "what's reachable locally" block
// from the active port-forwards, sorted by service for deterministic output.
// Returns "" when there are no forwards (nothing to print).
func formatAccessSummary(forwards []activeForward) string {
	if len(forwards) == 0 {
		return ""
	}

	fwds := append([]activeForward(nil), forwards...)
	sort.Slice(fwds, func(i, j int) bool {
		if fwds[i].Service != fwds[j].Service {
			return fwds[i].Service < fwds[j].Service
		}
		return fwds[i].Local < fwds[j].Local
	})

	// Address uses the local (forwarded) port; scheme is decided by the service
	// port (accessURL), so a conflict-shifted DB still reads as a bare host:port.
	urls := make([]string, len(fwds))
	svcW, urlW := 0, 0
	for i, f := range fwds {
		urls[i] = accessURL(f.Local, f.Port)
		if len(f.Service) > svcW {
			svcW = len(f.Service)
		}
		if len(urls[i]) > urlW {
			urlW = len(urls[i])
		}
	}

	var b strings.Builder
	b.WriteString("\n🔗 Services available locally (active until Ctrl+C):\n")
	for i, f := range fwds {
		fmt.Fprintf(&b, "     %-*s   %-*s   → svc/%s:%d\n", svcW, f.Service, urlW, urls[i], f.Service, f.Port)
	}
	return b.String()
}

// AccessSummary collects every node's active port-forwards into one block,
// printed after the DAG comes up so developers see exactly what to hit.
func (d *DAG) AccessSummary() string {
	var all []activeForward
	for _, n := range d.Nodes {
		all = append(all, n.forwards...)
	}
	return formatAccessSummary(all)
}

// formatTeardown renders the teardown block for one deleted kubernetes/helm
// node: a header naming the release + namespace, then the deleted-resource lines
// from the underlying `kubectl delete` / `helm uninstall` output (indented,
// blank lines dropped) so the developer sees exactly what was removed.
func formatTeardown(name, ns, kubectlOut string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🧹 Tearing down '%s' (ns/%s)...\n", name, ns)
	for _, line := range strings.Split(kubectlOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fmt.Fprintf(&b, "     %s\n", line)
	}
	return b.String()
}
