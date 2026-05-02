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

package logs

import (
	"os"
	"sort"
	"strings"
)

// SecretRedactor replaces known secret values in log output with a redaction
// marker. Because devx centrally loads all secrets via vault/env integration,
// it can perform exact-match replacement rather than relying on pattern-based
// heuristics.
type SecretRedactor struct {
	// secrets is sorted longest-first to prevent partial matches when a
	// shorter secret is a substring of a longer one.
	secrets []string
}

const redactedPlaceholder = "[REDACTED]"

// NewSecretRedactor builds a redactor from the current environment. It
// collects all env-var values and filters out obviously non-sensitive short
// values (length <= 3) and common system paths to avoid false positives.
func NewSecretRedactor() *SecretRedactor {
	return NewSecretRedactorFromPairs(os.Environ())
}

// NewSecretRedactorFromPairs builds a redactor from explicit KEY=VALUE pairs.
// Useful for testing or when secrets come from a vault loader rather than the
// process environment.
func NewSecretRedactorFromPairs(pairs []string) *SecretRedactor {
	seen := make(map[string]bool)
	var secrets []string

	for _, kv := range pairs {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		val := parts[1]

		// Skip empty, very short, or obviously non-secret values
		if len(val) <= 3 {
			continue
		}

		// Skip known non-sensitive env vars
		if isNonSensitiveKey(key) {
			continue
		}

		if !seen[val] {
			seen[val] = true
			secrets = append(secrets, val)
		}
	}

	// Sort longest-first so that a longer secret containing a shorter
	// substring is replaced first, preventing partial matches.
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})

	return &SecretRedactor{secrets: secrets}
}

// Redact replaces all known secret values in the input string.
func (r *SecretRedactor) Redact(s string) string {
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, redactedPlaceholder)
	}
	return s
}

// isNonSensitiveKey returns true for environment variable names that are
// universally non-secret (paths, locales, terminal settings, etc.).
func isNonSensitiveKey(key string) bool {
	nonSensitive := map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "SHELL": true,
		"TERM": true, "LANG": true, "LC_ALL": true, "LC_CTYPE": true,
		"EDITOR": true, "VISUAL": true, "PAGER": true, "LESS": true,
		"PWD": true, "OLDPWD": true, "SHLVL": true, "LOGNAME": true,
		"HOSTNAME": true, "TMPDIR": true, "XDG_DATA_HOME": true,
		"XDG_CONFIG_HOME": true, "XDG_CACHE_HOME": true, "XDG_RUNTIME_DIR": true,
		"COLORTERM": true, "TERM_PROGRAM": true, "TERM_PROGRAM_VERSION": true,
		"GOPATH": true, "GOROOT": true, "GOBIN": true,
		"NVM_DIR": true, "NVM_BIN": true,
		"SSH_AUTH_SOCK": true, "SSH_AGENT_PID": true,
		"DISPLAY": true, "WAYLAND_DISPLAY": true,
		"_": true,
	}

	return nonSensitive[key]
}
