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

//go:build integration

//	Run: DEVX_NATIVE_ITEST_HOST=<ssh-host> go test -tags integration \
//	  ./internal/multinode/provider/ -run NativeIntegration -timeout 10m
//
// Exercises the native provider against a real Linux host over SSH: EnsureRuntime
// (deps/firewall/tailscale) returns a node IP, and IsInstalled reflects K3s state.
// Opt-in (build tag + env) so it never runs in the normal suite.
package provider

import (
	"context"
	"os"
	"testing"

	"github.com/VitruvianSoftware/devx/internal/multinode/config"
)

func TestNativeIntegration(t *testing.T) {
	host := os.Getenv("DEVX_NATIVE_ITEST_HOST")
	if host == "" {
		t.Skip("set DEVX_NATIVE_ITEST_HOST to a reachable Linux SSH host")
	}
	p := NewNativeProvider(config.NodeConfig{Host: host, Kind: "native", Role: "agent", Pool: "linux"}, &config.Config{})
	ip, err := p.EnsureRuntime(context.Background())
	if err != nil || ip == "" {
		t.Fatalf("EnsureRuntime(%s) = %q, %v", host, ip, err)
	}
	t.Logf("native host %s node-ip=%s", host, ip)
	if _, err := p.IsInstalled(context.Background()); err != nil {
		t.Errorf("IsInstalled: %v", err)
	}
}
