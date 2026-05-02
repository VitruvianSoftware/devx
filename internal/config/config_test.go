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

package config_test

import (
	"testing"

	"github.com/VitruvianSoftware/devx/internal/config"
)

func TestNew_Defaults(t *testing.T) {
	cfg := config.New("james", "", "", "")

	if cfg.DevHostname != "james-dev-machine" {
		t.Errorf("DevHostname = %q, want %q", cfg.DevHostname, "james-dev-machine")
	}
	if cfg.CFDomain != "james.ipv1337.dev" {
		t.Errorf("CFDomain = %q, want %q", cfg.CFDomain, "james.ipv1337.dev")
	}
	if cfg.TunnelName != "dev-tunnel-james" {
		t.Errorf("TunnelName = %q, want %q", cfg.TunnelName, "dev-tunnel-james")
	}
}

func TestNew_ExplicitOverrides(t *testing.T) {
	cfg := config.New("james", "custom-host", "custom-tunnel", "custom.domain.dev")

	if cfg.DevHostname != "custom-host" {
		t.Errorf("DevHostname = %q, want %q", cfg.DevHostname, "custom-host")
	}
	if cfg.TunnelName != "custom-tunnel" {
		t.Errorf("TunnelName = %q, want %q", cfg.TunnelName, "custom-tunnel")
	}
	if cfg.CFDomain != "custom.domain.dev" {
		t.Errorf("CFDomain = %q, want %q", cfg.CFDomain, "custom.domain.dev")
	}
}

func TestValidate_MissingDevName(t *testing.T) {
	cfg := &config.Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty DevName, got nil")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := config.New("james", "", "", "")
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
