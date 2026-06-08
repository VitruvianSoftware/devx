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
