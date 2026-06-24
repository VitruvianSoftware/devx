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
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/remote"
)

// A Manager built with NewManagerWithExec routes commands through the injected
// exec (used by the native provider), not limactl.
func TestManagerWithExec_RoutesThroughExec(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("fedora"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		if strings.Contains(cmd, "node-token") {
			return "tok::123", nil
		}
		return "", nil
	})
	tok, err := m.GetToken(context.Background())
	if err != nil || tok != "tok::123" {
		t.Fatalf("GetToken via exec = %q,%v", tok, err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "/var/lib/rancher/k3s/server/node-token") {
		t.Errorf("exec should have run the node-token read; got %v", got)
	}
}

// InstallTailscale must ALWAYS enforce --accept-routes idempotently. The bug:
// `tailscale up --accept-routes` is skipped if the node was already up (joined
// previously WITHOUT accept-routes), leaving RouteAll=false (this broke the
// Fedora nodes on the Cilium native-routing migration). An explicit
// `tailscale set --accept-routes=true` after `up` enforces it regardless.
func TestInstallTailscale_EnforcesAcceptRoutes(t *testing.T) {
	var got []string
	m := NewManagerWithExec(remote.NewRunner("fedora"), func(ctx context.Context, cmd string) (string, error) {
		got = append(got, cmd)
		if strings.Contains(cmd, "tailscale ip -4") {
			return "100.97.82.15\n", nil
		}
		return "", nil
	})
	ip, err := m.InstallTailscale(context.Background(), "tskey-abc")
	if err != nil {
		t.Fatalf("InstallTailscale: %v", err)
	}
	if ip != "100.97.82.15" {
		t.Errorf("tailscale ip = %q", ip)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "tailscale set --accept-routes=true") {
		t.Errorf("InstallTailscale must run `tailscale set --accept-routes=true`; got:\n%s", joined)
	}
}
