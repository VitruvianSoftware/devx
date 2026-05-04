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

package ai

import (
	"strings"
	"testing"
)

func TestMatchRule_PasswordAuth(t *testing.T) {
	result := matchRule("FATAL: password authentication failed for user \"devx\"", 0)
	if result == "" {
		t.Fatal("expected a diagnosis for password auth failure")
	}
	if !strings.Contains(result, "credentials mismatch") {
		t.Errorf("expected diagnosis to mention credentials mismatch, got: %s", result)
	}
}

func TestMatchRule_AddressInUse(t *testing.T) {
	result := matchRule("listen tcp :8080: bind: address already in use", 0)
	if result == "" {
		t.Fatal("expected a diagnosis for address in use")
	}
	if !strings.Contains(result, "Port conflict") {
		t.Errorf("expected diagnosis to mention port conflict, got: %s", result)
	}
}

func TestMatchRule_ConnectionRefused(t *testing.T) {
	result := matchRule("dial tcp 127.0.0.1:5432: connection refused", 0)
	if result == "" {
		t.Fatal("expected a diagnosis for connection refused")
	}
}

func TestMatchRule_NoMatch(t *testing.T) {
	result := matchRule("some completely random error nobody has seen before", 0)
	if result != "" {
		t.Errorf("expected no match for unknown error, got: %s", result)
	}
}

func TestMatchRule_CaseInsensitive(t *testing.T) {
	result := matchRule("ADDRESS ALREADY IN USE", 0)
	if result == "" {
		t.Fatal("expected case-insensitive matching to work")
	}
}

func TestMatchRule_OOMKilled(t *testing.T) {
	result := matchRule("container was OOMKilled after exceeding memory limits", 0)
	if result == "" {
		t.Fatal("expected a diagnosis for OOMKilled")
	}
	if !strings.Contains(result, "memory") {
		t.Errorf("expected diagnosis to mention memory, got: %s", result)
	}
}

func TestMatchRule_ExecFormatError(t *testing.T) {
	result := matchRule("exec format error", 0)
	if result == "" {
		t.Fatal("expected a diagnosis for exec format error")
	}
	if !strings.Contains(result, "Architecture") {
		t.Errorf("expected diagnosis to mention architecture, got: %s", result)
	}
}

func TestDiagnoseFailure_RuleBasedOnly(t *testing.T) {
	// DiagnoseFailure should return a rule-based diagnosis without AI
	result := DiagnoseFailure("db spawn postgres", 1, "address already in use", "")
	if result == "" {
		t.Fatal("expected a diagnosis from DiagnoseFailure")
	}
	if !strings.Contains(result, "Port conflict") {
		t.Errorf("expected diagnosis to mention port conflict, got: %s", result)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
