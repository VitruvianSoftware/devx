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

package provider

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

// #365: load-bearing host-convergence steps must FAIL LOUDLY, not be silently
// swallowed (`_, _ = p.run(...)`). A broken firewalld trust means REJECTed pod
// networking and a missing accept-routes breaks Cilium-over-tailscale routing —
// in both cases the join/reconcile must not report success.

func TestNative_EnsureHostPrereqs_FirewalldTrustFailureIsLoud(t *testing.T) {
	p := NewNativeProvider(config.NodeConfig{Host: "fedora", Kind: "native", Role: "agent"}, &config.Config{})
	p.run = func(_ context.Context, cmd string) (string, error) {
		switch {
		case strings.Contains(cmd, "is-enabled firewalld"):
			return "enabled", nil
		case strings.Contains(cmd, "--zone=trusted"):
			return "", errors.New("firewalld rejected the rule")
		default:
			return "", nil
		}
	}
	if err := p.ensureHostPrereqs(context.Background()); err == nil {
		t.Fatal("ensureHostPrereqs must fail loudly when the firewalld pod-network trust command fails")
	}
}

func TestNative_EnsureRuntime_AcceptRoutesFailureIsLoud(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cluster.Tailscale.Enabled = true
	cfg.Cluster.Tailscale.AuthKey = "tskey-abc123"
	p := NewNativeProvider(config.NodeConfig{Host: "fedora", Kind: "native", Role: "agent"}, cfg)
	p.run = func(_ context.Context, cmd string) (string, error) {
		if strings.Contains(cmd, "tailscale set --accept-routes") {
			return "", errors.New("tailscaled not responding")
		}
		return "", nil
	}
	if _, err := p.EnsureRuntime(context.Background()); err == nil {
		t.Fatal("EnsureRuntime must fail loudly when --accept-routes cannot be enforced")
	}
}
