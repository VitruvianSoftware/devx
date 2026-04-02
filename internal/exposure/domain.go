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
