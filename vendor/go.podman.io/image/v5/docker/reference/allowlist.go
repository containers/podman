package reference

import (
	"strings"
	"sync/atomic"
)

// additionalDefaultDomains stores extra domains that should be treated as fully
// qualified even if they do not contain a dot or port number.
var additionalDefaultDomains atomic.Value // map[string]struct{}

func init() {
	additionalDefaultDomains.Store(map[string]struct{}{})
}

// SetAdditionalDefaultDomains configures extra registry domains that should be
// considered fully qualified during image reference parsing.
func SetAdditionalDefaultDomains(domains []string) {
	allowlist := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		domain = strings.TrimSpace(strings.ToLower(domain))
		if domain == "" {
			continue
		}
		allowlist[domain] = struct{}{}
	}
	additionalDefaultDomains.Store(allowlist)
}

func isAdditionalDefaultDomain(domain string) bool {
	allowlist := additionalDefaultDomains.Load().(map[string]struct{})
	_, ok := allowlist[strings.ToLower(domain)]
	return ok
}
