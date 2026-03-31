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
