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

// SecurityPolicyIDFromPath returns the security policy id from a canonical policy path
// (/infra/domains/{domain}/security-policies/{id}) or a rule path (.../security-policies/{id}/rules/...).
func SecurityPolicyIDFromPath(path string) string {
	p := NormalizeResourceKey(path)
	parts := strings.Split(strings.TrimPrefix(p, "/"), "/")
	if len(parts) < 5 {
		return ""
	}
	if parts[0] != "infra" || parts[1] != "domains" || parts[3] != "security-policies" {
		return ""
	}
	return parts[4]
}
