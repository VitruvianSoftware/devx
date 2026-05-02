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
