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

package mcpinstall

import (
	"sort"
	"testing"
)

func TestHostsIsNonEmpty(t *testing.T) {
	if len(Hosts()) == 0 {
		t.Fatal("Hosts() returned empty registry")
	}
}

func TestEveryHostHasNonEmptyKeyAndName(t *testing.T) {
	for _, h := range Hosts() {
		if h.Key() == "" {
			t.Errorf("host with empty key: %T", h)
		}
		if h.DisplayName() == "" {
			t.Errorf("host %q has empty DisplayName", h.Key())
		}
	}
}

func TestHostKeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, h := range Hosts() {
		if seen[h.Key()] {
			t.Errorf("duplicate host key: %s", h.Key())
		}
		seen[h.Key()] = true
	}
}

func TestHostByKeyRoundTrip(t *testing.T) {
	for _, h := range Hosts() {
		got := HostByKey(h.Key())
		if got == nil {
			t.Errorf("HostByKey(%q) = nil", h.Key())
		} else if got.Key() != h.Key() {
			t.Errorf("HostByKey(%q) returned %q", h.Key(), got.Key())
		}
	}
	if HostByKey("does-not-exist") != nil {
		t.Errorf("HostByKey should return nil for unknown key")
	}
}

func TestAllHostKeysIsSorted(t *testing.T) {
	keys := AllHostKeys()
	if !sort.StringsAreSorted(keys) {
		t.Errorf("AllHostKeys not sorted: %v", keys)
	}
}

func TestResolveTargetsByName(t *testing.T) {
	hosts, err := ResolveTargets([]string{"claude-code", "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 {
		t.Errorf("got %d hosts, want 2", len(hosts))
	}
}

func TestResolveTargetsUnknownErrors(t *testing.T) {
	_, err := ResolveTargets([]string{"not-a-real-agent"})
	if err == nil {
		t.Errorf("expected error for unknown host")
	}
}

func TestExpectedHostsAreRegistered(t *testing.T) {
	wantKeys := []string{
		"claude-code", "cursor", "codex", "gemini-cli",
		"zed", "continue", "windsurf", "opencode",
		"cowork", "antigravity",
	}
	got := map[string]bool{}
	for _, h := range Hosts() {
		got[h.Key()] = true
	}
	for _, k := range wantKeys {
		if !got[k] {
			t.Errorf("expected host %q to be registered", k)
		}
	}
}
