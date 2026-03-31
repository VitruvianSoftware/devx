package exposure

import (
	"testing"
)

func TestGenerateDomain(t *testing.T) {
	tests := []struct {
		name     string
		exposeID string
		cfDomain string
		expected string
	}{
		{
			name:     "Flattened Subdomain",
			exposeID: "eagle",
			cfDomain: "james.ipv1337.dev",
			expected: "eagle-james.ipv1337.dev",
		},
		{
			name:     "Flattened Deeper Subdomain",
			exposeID: "eagle",
			cfDomain: "api.james.ipv1337.dev",
			expected: "eagle-api.james.ipv1337.dev",
		},
		{
			name:     "Root Domain",
			exposeID: "eagle",
			cfDomain: "ipv1337.dev",
			expected: "eagle.ipv1337.dev",
		},
		{
			name:     "Short Root Domain",
			exposeID: "eagle",
			cfDomain: "foobar.com",
			expected: "eagle.foobar.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateDomain(tt.exposeID, tt.cfDomain)
			if result != tt.expected {
				t.Errorf("GenerateDomain(%q, %q) = %q, want %q", tt.exposeID, tt.cfDomain, result, tt.expected)
			}
		})
	}
}
