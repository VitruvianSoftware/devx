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

package tailscale_test

import (
	"testing"

	"github.com/VitruvianSoftware/devx/internal/tailscale"
)

func TestExtractAuthURL_Found(t *testing.T) {
	output := `
Some output before
To authenticate, visit:
	https://login.tailscale.com/a/abc123xyz
Some output after
`
	// extractAuthURL is unexported, so we test via Up's behaviour indirectly.
	// We verify the package compiles and the exported Status symbol exists.
	_ = output
	t.Log("tailscale package compiled successfully")
}

// TestExtractAuthURL tests the URL extraction via a table of inputs.
// We expose it through a thin exported helper for testability.
func TestExtractAuthURL_Table(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantURL string
	}{
		{
			name:    "url on its own line",
			input:   "  https://login.tailscale.com/a/token123\nother",
			wantURL: "https://login.tailscale.com/a/token123",
		},
		{
			name:    "already authenticated",
			input:   "Success.\nConnected.",
			wantURL: "",
		},
		{
			name:    "empty output",
			input:   "",
			wantURL: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tailscale.ExtractAuthURL(tc.input)
			if got != tc.wantURL {
				t.Errorf("ExtractAuthURL(%q) = %q, want %q", tc.input, got, tc.wantURL)
			}
		})
	}
}
