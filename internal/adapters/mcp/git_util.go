package mcp

import (
	"regexp"
	"strings"
)

var gitURLIDRegexp = regexp.MustCompile(`[^a-z0-9-]+`)

// gitIDFromURLWithFallback derives a stable KB/spec id from a git URL. Strips
// trailing slash and `.git`, takes the last path segment (after `/` or `:`),
// lowercases it, and replaces non-alphanumeric-dash runs with `-`. Returns
// `fallback` when the derivation produces an empty string.
func gitIDFromURLWithFallback(url, fallback string) string {
	u := strings.TrimSuffix(strings.TrimRight(url, "/"), ".git")
	seg := u[strings.LastIndexAny(u, "/:")+1:]
	seg = gitURLIDRegexp.ReplaceAllString(strings.ToLower(seg), "-")
	seg = strings.Trim(seg, "-")
	if seg == "" {
		return fallback
	}
	return seg
}
