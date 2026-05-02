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

package remote

import (
	"context"
	"strings"
	"testing"
)

func TestIsTransient(t *testing.T) {
	tests := []struct {
		errMsg    string
		transient bool
	}{
		{"Connection refused", true},
		{"Connection reset by peer", true},
		{"Connection timed out", true},
		{"No route to host", true},
		{"Host is down", true},
		{"Network is unreachable", true},
		{"ssh_exchange_identification: read: Connection reset by peer", true},
		{"kex_exchange_identification: write: Broken pipe", true},
		{"Permission denied (publickey)", false},
		{"command not found: kubectl", false},
		{"exit status 1", false},
	}

	for _, tt := range tests {
		t.Run(tt.errMsg, func(t *testing.T) {
			got := isTransient(tt.errMsg)
			if got != tt.transient {
				t.Errorf("isTransient(%q) = %v, want %v", tt.errMsg, got, tt.transient)
			}
		})
	}
}

func TestNewRunner(t *testing.T) {
	r := NewRunner("test-host")
	if r.Host != "test-host" {
		t.Errorf("expected host 'test-host', got %q", r.Host)
	}
	if r.MaxRetries != DefaultMaxRetries {
		t.Errorf("expected max retries %d, got %d", DefaultMaxRetries, r.MaxRetries)
	}
}

func TestNewRunnerWithOpts(t *testing.T) {
	r := NewRunnerWithOpts("host", "user", "2222", "/path/to/key")
	if r.Host != "host" {
		t.Errorf("expected host 'host', got %q", r.Host)
	}
	if r.User != "user" {
		t.Errorf("expected user 'user', got %q", r.User)
	}
	if r.Port != "2222" {
		t.Errorf("expected port '2222', got %q", r.Port)
	}
	if r.KeyPath != "/path/to/key" {
		t.Errorf("expected key path '/path/to/key', got %q", r.KeyPath)
	}
}

func TestRun_CancelledContext(t *testing.T) {
	r := NewRunner("nonexistent-host")
	r.MaxRetries = 0

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := r.Run(ctx, "echo test")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got: %v", err)
	}
}
