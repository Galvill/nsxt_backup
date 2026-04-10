package dfw

import "strings"

// NormalizeResourceKey returns a portable key starting with /infra/...
// Strips leading org/project segments if present in API path strings.
func NormalizeResourceKey(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if i := strings.Index(p, "/infra/"); i >= 0 {
		p = p[i:]
	}
	return normalizePath(p)
}

// RelFromCanonical converts /infra/... to API relative path (no leading slash).
func RelFromCanonical(canonical string) string {
	c := NormalizeResourceKey(canonical)
	c = strings.TrimPrefix(c, "/")
	return c
}
