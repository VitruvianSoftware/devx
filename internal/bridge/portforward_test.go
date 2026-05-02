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

package bridge

import (
	"context"
	"testing"
	"time"
)

func TestPortForwardResolveLocalPort_Auto(t *testing.T) {
	pf := NewPortForward("/tmp/kc", "ctx", "default", "test-svc", 8080, 0)

	warning, err := pf.ResolveLocalPort()
	if err != nil {
		t.Fatalf("ResolveLocalPort: %v", err)
	}
	if warning != "" {
		t.Errorf("unexpected warning: %s", warning)
	}
	if pf.LocalPort == 0 {
		t.Error("LocalPort should be assigned a free port, still 0")
	}
	if pf.LocalPort < 1024 || pf.LocalPort > 65535 {
		t.Errorf("LocalPort %d is out of valid range", pf.LocalPort)
	}
}

func TestPortForwardLocalAddr(t *testing.T) {
	pf := NewPortForward("/tmp/kc", "ctx", "default", "test-svc", 8080, 9501)
	got := pf.LocalAddr()
	want := "127.0.0.1:9501"
	if got != want {
		t.Errorf("LocalAddr() = %q, want %q", got, want)
	}
}

func TestPortForwardStateTransitions(t *testing.T) {
	pf := NewPortForward("/tmp/kc", "ctx", "default", "test-svc", 8080, 9501)

	if pf.State() != StateStarting {
		t.Errorf("initial state = %v, want StateStarting", pf.State())
	}

	pf.Stop()
	// Give the state channel a moment to process
	time.Sleep(10 * time.Millisecond)

	if pf.State() != StateStopped {
		t.Errorf("after Stop(), state = %v, want StateStopped", pf.State())
	}
}

func TestPortForwardStartWithCancelledContext(t *testing.T) {
	pf := NewPortForward("/tmp/nonexistent-kc", "ctx", "default", "test-svc", 8080, 0)
	_, _ = pf.ResolveLocalPort()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := pf.Start(ctx)
	if err != nil {
		t.Errorf("Start with cancelled context should return nil, got: %v", err)
	}
	if pf.State() != StateStopped {
		t.Errorf("state after cancelled Start = %v, want StateStopped", pf.State())
	}
}

func TestPortForwardStateString(t *testing.T) {
	tests := []struct {
		state PortForwardState
		want  string
	}{
		{StateStarting, "starting"},
		{StateHealthy, "healthy"},
		{StateFailed, "failed"},
		{StateStopped, "stopped"},
		{PortForwardState(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("PortForwardState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
