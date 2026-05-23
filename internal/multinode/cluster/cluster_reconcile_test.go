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

package cluster

import (
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/lima"
)

func TestEnsureDepsCommand(t *testing.T) {
	cmd := ensureDepsCommand()

	// It must perform an idempotent apt install of the standard node package
	// set. LimaShellSudo wraps this in `sudo sh -c`, so it must carry no sudo
	// prefix of its own.
	for _, want := range []string{"apt-get update", "apt-get install", "-y"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("ensureDepsCommand() = %q, want it to contain %q", cmd, want)
		}
	}
	if strings.Contains(cmd, "sudo") {
		t.Errorf("ensureDepsCommand() = %q, should not contain its own sudo (LimaShellSudo adds it)", cmd)
	}

	// It must install exactly the shared package list — so reconcile converges
	// existing nodes to the same baseline GenerateConfig provisions new ones.
	if !strings.Contains(cmd, lima.NodePackages) {
		t.Errorf("ensureDepsCommand() = %q, want it to install lima.NodePackages %q", cmd, lima.NodePackages)
	}
}

func TestNodePackagesIncludesSocat(t *testing.T) {
	// socat is the reason reconcile exists: kubectl port-forward and devx
	// bridge need it on Docker-runtime k3s nodes. Guard against it being
	// dropped from the baseline.
	if !strings.Contains(lima.NodePackages, "socat") {
		t.Errorf("lima.NodePackages = %q, must include socat for port-forward/bridge", lima.NodePackages)
	}
}
