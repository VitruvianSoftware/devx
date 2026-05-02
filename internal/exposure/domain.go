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
	"fmt"
	"strings"
)

// GenerateDomain constructs a flattened subdomain to prevent SSL wildcard mismatch.
// Cloudflare Universal SSL certificates automatically cover 1 level of wildcard (*.example.com).
// If cfDomain is already a subdomain (e.g. james.ipv1337.dev - 3 parts), navigating one level deeper
// (eagle.james.ipv1337.dev) causes SSL errors (ERR_SSL_VERSION_OR_CIPHER_MISMATCH) because it requires
// Advanced Certificate Manager coverage (*.*.ipv1337.dev is invalid, you must cover the base specifically).
// To bypass this, we combine the exposeID and the first subdomain part with a hyphen.
func GenerateDomain(exposeID, cfDomain string) string {
	parts := strings.Split(cfDomain, ".")
	if len(parts) > 2 {
		sub := parts[0]
		rest := strings.Join(parts[1:], ".")
		return fmt.Sprintf("%s-%s.%s", exposeID, sub, rest)
	}
	return fmt.Sprintf("%s.%s", exposeID, cfDomain)
}
