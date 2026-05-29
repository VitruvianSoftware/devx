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
	"strings"
	"testing"
)

// accessURL: the address uses the local (forwarded) port; the scheme is decided
// by the SERVICE port (the stable protocol indicator) — HTTP for web ports, bare
// host:port for known non-HTTP (database / broker) ports. Keying the scheme off
// the service port means a DB whose local port was conflict-shifted still reads
// as a bare host:port, not a misleading http:// URL.
func TestAccessURL(t *testing.T) {
	cases := []struct {
		local, svc int
		want       string
	}{
		{8080, 8080, "http://localhost:8080"},
		{4443, 4443, "http://localhost:4443"},
		{3000, 3000, "http://localhost:3000"},
		{5432, 5432, "localhost:5432"},        // postgres, no shift
		{5433, 5432, "localhost:5433"},        // postgres shifted off 5432 → still bare
		{8081, 8080, "http://localhost:8081"}, // http shifted → still http
		{3306, 3306, "localhost:3306"},        // mysql
		{6379, 6379, "localhost:6379"},        // redis
		{27017, 27017, "localhost:27017"},     // mongodb
	}
	for _, c := range cases {
		if got := accessURL(c.local, c.svc); got != c.want {
			t.Errorf("accessURL(%d, %d) = %q, want %q", c.local, c.svc, got, c.want)
		}
	}
}

// formatAccessSummary renders the consolidated "what's reachable locally" block
// from the active port-forwards; empty input prints nothing.
func TestFormatAccessSummary(t *testing.T) {
	if got := formatAccessSummary(nil); got != "" {
		t.Errorf("formatAccessSummary(nil) = %q, want empty", got)
	}

	fwds := []activeForward{
		{Local: 8080, Service: "brms-offer", Port: 8080},
		{Local: 5433, Service: "brms-db-rw", Port: 5432}, // local port conflict-shifted off 5432
		{Local: 8081, Service: "brms-exec", Port: 8080},
	}
	out := formatAccessSummary(fwds)

	if !strings.Contains(out, "Services available locally") {
		t.Errorf("missing header:\n%s", out)
	}
	// Web services → http URL + the svc/port mapping.
	for _, want := range []string{
		"http://localhost:8080", "svc/brms-offer:8080",
		"http://localhost:8081", "svc/brms-exec:8080",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// Postgres → bare host:port at its (shifted) local port, never an http URL —
	// scheme follows the service port (5432), not the local one (5433).
	if !strings.Contains(out, "localhost:5433") {
		t.Errorf("missing postgres address:\n%s", out)
	}
	if strings.Contains(out, "http://localhost:5433") {
		t.Errorf("postgres should not be an http URL:\n%s", out)
	}
	// Deterministic: sorted by service name (brms-db-rw < brms-exec < brms-offer).
	iDB := strings.Index(out, "brms-db-rw")
	iExec := strings.Index(out, "brms-exec")
	iOffer := strings.Index(out, "brms-offer")
	if iDB >= iExec || iExec >= iOffer {
		t.Errorf("rows not sorted by service name (db=%d exec=%d offer=%d):\n%s", iDB, iExec, iOffer, out)
	}
}

// DAG.AccessSummary collects every node's active port-forwards into one block.
func TestDAGAccessSummary(t *testing.T) {
	d := NewDAG()
	kube := &Node{Name: "brms", Type: NodeService, Runtime: RuntimeKubernetes}
	kube.forwards = []activeForward{
		{Local: 8080, Service: "brms-offer", Port: 8080},
		{Local: 4443, Service: "fake-gcs", Port: 4443},
	}
	_ = d.AddNode(kube)
	_ = d.AddNode(&Node{Name: "noop", Type: NodeService, Runtime: RuntimeHost})

	out := d.AccessSummary()
	for _, want := range []string{"http://localhost:8080", "http://localhost:4443", "svc/fake-gcs:4443"} {
		if !strings.Contains(out, want) {
			t.Errorf("AccessSummary missing %q:\n%s", want, out)
		}
	}

	empty := NewDAG()
	_ = empty.AddNode(&Node{Name: "x", Type: NodeService, Runtime: RuntimeHost})
	if got := empty.AccessSummary(); got != "" {
		t.Errorf("AccessSummary with no forwards = %q, want empty", got)
	}
}

// formatTeardown renders the per-node teardown block: a header naming the
// release + namespace, then kubectl's deleted-resource lines (indented, blanks
// dropped).
func TestFormatTeardown(t *testing.T) {
	out := formatTeardown("brms", "brms-dev", `service "brms-offer" deleted
deployment.apps "brms-offer" deleted

service "fake-gcs" deleted
`)
	if !strings.Contains(out, "brms") || !strings.Contains(out, "brms-dev") {
		t.Errorf("header missing name/ns:\n%s", out)
	}
	for _, want := range []string{
		`service "brms-offer" deleted`,
		`deployment.apps "brms-offer" deleted`,
		`service "fake-gcs" deleted`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing resource line %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, " deleted"); n != 3 {
		t.Errorf("want 3 deleted-resource lines, got %d:\n%s", n, out)
	}

	// Empty kubectl output (e.g. --ignore-not-found removed nothing) → header
	// still shown so the developer sees the teardown happened.
	hdrOnly := formatTeardown("brms", "brms-dev", "  \n  ")
	if !strings.Contains(hdrOnly, "brms-dev") {
		t.Errorf("expected header for empty output, got:\n%s", hdrOnly)
	}
	if strings.Contains(hdrOnly, " deleted") {
		t.Errorf("expected no resource lines for empty output, got:\n%s", hdrOnly)
	}
}
