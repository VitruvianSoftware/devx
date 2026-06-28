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

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// #371: Tier-3 alpha commands must be gated behind --experimental /
// DEVX_EXPERIMENTAL so the GA core isn't diluted by accidental invocation.
func TestExperimentalGate(t *testing.T) {
	orig := Experimental
	t.Cleanup(func() { Experimental = orig })

	c := &cobra.Command{Use: "x", Short: "do x"}
	markExperimental(c, "devx x")

	if !strings.Contains(c.Short, "[experimental]") {
		t.Errorf("Short should be tagged [experimental], got %q", c.Short)
	}

	// Off by default → gated. (t.Setenv auto-restores the env after the test.)
	Experimental = false
	t.Setenv("DEVX_EXPERIMENTAL", "")
	if err := c.PersistentPreRunE(c, nil); err == nil {
		t.Error("must be gated when --experimental is off and env empty")
	}

	// --experimental flag unlocks.
	Experimental = true
	if err := c.PersistentPreRunE(c, nil); err != nil {
		t.Errorf("--experimental must unlock: %v", err)
	}

	// DEVX_EXPERIMENTAL truthy unlocks.
	Experimental = false
	t.Setenv("DEVX_EXPERIMENTAL", "1")
	if err := c.PersistentPreRunE(c, nil); err != nil {
		t.Errorf("DEVX_EXPERIMENTAL=1 must unlock: %v", err)
	}

	// DEVX_EXPERIMENTAL=0 stays gated.
	t.Setenv("DEVX_EXPERIMENTAL", "0")
	if err := c.PersistentPreRunE(c, nil); err == nil {
		t.Error("DEVX_EXPERIMENTAL=0 must stay gated")
	}
}

// The brittle Tier-3 alphas must actually carry the gate; the actively-used
// cluster + bridge suites must NOT.
func TestExperimentalGate_AppliedToAlphasNotCore(t *testing.T) {
	for _, c := range []*cobra.Command{cloudCmd, k8sCmd, mockCmd, mailCmd} {
		if c.PersistentPreRunE == nil {
			t.Errorf("%q should be gated experimental", c.Name())
		}
	}
	for _, c := range []*cobra.Command{clusterMgmtCmd, bridgeCmd} {
		if c.PersistentPreRunE != nil {
			t.Errorf("%q is in active use and must NOT be gated", c.Name())
		}
	}
}
