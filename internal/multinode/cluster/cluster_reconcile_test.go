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

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
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

func TestNodeChanges(t *testing.T) {
	vm := config.VMConfig{CPUs: 2, Memory: "4GiB", Disk: "30GiB"}
	matchingSpec := lima.Spec{CPUs: 2, Memory: "4GiB", Disk: "30GiB", Mounts: []config.MountConfig{{Location: "~", Writable: true}}}
	desiredHome := lima.ResolveMounts(nil) // [{~ writable}]

	cases := []struct {
		name              string
		spec              lima.Spec
		vm                config.VMConfig
		desired           []config.MountConfig
		wantHW, wantMount bool
	}{
		{"all equal -> no change", matchingSpec, vm, desiredHome, false, false},
		{"cpu differs", lima.Spec{CPUs: 4, Memory: "4GiB", Disk: "30GiB", Mounts: desiredHome}, vm, desiredHome, true, false},
		{"memory differs", lima.Spec{CPUs: 2, Memory: "8GiB", Disk: "30GiB", Mounts: desiredHome}, vm, desiredHome, true, false},
		{"disk differs", lima.Spec{CPUs: 2, Memory: "4GiB", Disk: "50GiB", Mounts: desiredHome}, vm, desiredHome, true, false},
		// The reported bug: an existing VM with NO mounts vs the default home mount.
		{"no mounts vs default home", lima.Spec{CPUs: 2, Memory: "4GiB", Disk: "30GiB", Mounts: nil}, vm, desiredHome, false, true},
		{"both hw and mounts differ", lima.Spec{CPUs: 1, Memory: "2GiB", Disk: "10GiB", Mounts: nil}, vm, desiredHome, true, true},
	}
	for _, c := range cases {
		hw, mounts := nodeChanges(c.spec, c.vm, c.desired)
		if hw != c.wantHW || mounts != c.wantMount {
			t.Errorf("%s: nodeChanges = (hw=%v, mounts=%v), want (hw=%v, mounts=%v)", c.name, hw, mounts, c.wantHW, c.wantMount)
		}
	}
}
