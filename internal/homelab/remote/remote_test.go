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
