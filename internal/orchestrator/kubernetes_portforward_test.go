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

import "testing"

func TestParseServiceForwards(t *testing.T) {
	jsonOut := []byte(`{"items":[
		{"metadata":{"name":"api"},"spec":{"ports":[{"port":8080},{"port":9090}]}},
		{"metadata":{"name":"db"},"spec":{"ports":[{"port":5432}]}},
		{"metadata":{"name":"headless"},"spec":{"ports":[]}}
	]}`)
	fwds, err := parseServiceForwards(jsonOut)
	if err != nil {
		t.Fatalf("parseServiceForwards: %v", err)
	}
	want := []serviceForward{{"api", 8080}, {"api", 9090}, {"db", 5432}}
	if len(fwds) != len(want) {
		t.Fatalf("got %d forwards, want %d: %v", len(fwds), len(want), fwds)
	}
	for i := range want {
		if fwds[i] != want[i] {
			t.Errorf("fwd[%d] = %v, want %v", i, fwds[i], want[i])
		}
	}

	if empty, _ := parseServiceForwards([]byte(`{"items":[]}`)); len(empty) != 0 {
		t.Errorf("expected no forwards, got %v", empty)
	}
	if _, err := parseServiceForwards([]byte("not json")); err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestAssignLocalPorts(t *testing.T) {
	fwds := []serviceForward{
		{Service: "brms-offer", Port: 8080},
		{Service: "brms-exec", Port: 8080},
		{Service: "brms-mgmt", Port: 8080},
		{Service: "fake-gcs", Port: 4443},
	}
	// A resolve that never shifts for OS reasons (returns its input) isolates the
	// in-batch de-duplication: even so, the three :8080 services must each land
	// on a distinct local port. (kubectl forwards bind asynchronously, so an
	// OS-only check hands out 8080 three times — the bug this guards against.)
	active := assignLocalPorts(fwds, func(p int) int { return p })
	if len(active) != len(fwds) {
		t.Fatalf("got %d forwards, want %d", len(active), len(fwds))
	}
	seen := map[int]bool{}
	for _, a := range active {
		if seen[a.Local] {
			t.Errorf("duplicate local port %d (service %s)", a.Local, a.Service)
		}
		seen[a.Local] = true
	}
	// First claimant keeps the requested port; service/target are preserved.
	if active[0] != (activeForward{Local: 8080, Service: "brms-offer", Port: 8080}) {
		t.Errorf("first forward = %+v, want {8080 brms-offer 8080}", active[0])
	}
	// A non-conflicting port is left untouched.
	if active[3].Local != 4443 {
		t.Errorf("fake-gcs local = %d, want 4443", active[3].Local)
	}
}
