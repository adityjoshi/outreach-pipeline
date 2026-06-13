package util

import (
	"net/url"
	"strings"
)

// NormalizeDomain strips scheme, path, and www prefix from a domain input.
func NormalizeDomain(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			raw = u.Host
		}
	}
	raw = strings.TrimPrefix(raw, "www.")
	raw = strings.Split(raw, "/")[0]
	return raw
}
