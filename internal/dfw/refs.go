package dfw

import (
	"encoding/json"
	"strings"
)

// ExtractInfraPaths scans JSON (recursively) for strings that look like Policy infra paths.
func ExtractInfraPaths(raw []byte) []string {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	walkJSON(v, seen)
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out
}

func walkJSON(v interface{}, seen map[string]struct{}) {
	switch x := v.(type) {
	case string:
		for _, p := range pathsInString(x) {
			norm := normalizePath(p)
			if norm != "" {
				seen[norm] = struct{}{}
			}
		}
	case map[string]interface{}:
		for _, vv := range x {
			walkJSON(vv, seen)
		}
	case []interface{}:
		for _, vv := range x {
			walkJSON(vv, seen)
		}
	}
}

func pathsInString(s string) []string {
	var out []string
	for i := 0; i < len(s); {
		j := strings.Index(s[i:], "/infra/")
		if j < 0 {
			break
		}
		start := i + j
		end := start + 6
		for end < len(s) {
			c := s[end]
			if c <= ' ' || c == '"' || c == '\'' || c == ',' || c == '}' || c == ']' || c == ')' || c == '<' {
				break
			}
			end++
		}
		if end > start+6 {
			out = append(out, s[start:end])
		}
		i = start + 6
	}
	return out
}

func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/infra/") {
		return ""
	}
	if idx := strings.IndexAny(p, "?#"); idx >= 0 {
		p = p[:idx]
	}
	p = strings.TrimSuffix(p, "/")
	return p
}

// ResourceKind classifies a Policy path for restore ordering.
type ResourceKind int

const (
	KindUnknown ResourceKind = iota
	KindService
	KindGroup
	KindContextProfile
	KindSecurityPolicy
	KindRule
)

// ClassifyPath returns kind and true if path is a supported leaf resource.
func ClassifyPath(path string) (ResourceKind, bool) {
	path = normalizePath(path)
	if path == "" {
		return KindUnknown, false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "infra" {
		return KindUnknown, false
	}
	if parts[1] == "services" && len(parts) == 3 {
		return KindService, true
	}
	if parts[1] != "domains" || len(parts) < 5 {
		return KindUnknown, false
	}
	switch parts[3] {
	case "groups":
		if len(parts) == 5 {
			return KindGroup, true
		}
	case "context-profiles":
		if len(parts) == 5 {
			return KindContextProfile, true
		}
	case "security-policies":
		if len(parts) == 5 {
			return KindSecurityPolicy, true
		}
		if len(parts) == 7 && parts[5] == "rules" {
			return KindRule, true
		}
	}
	return KindUnknown, false
}

// KindPriority for topological restore order (lower first).
func KindPriority(k ResourceKind) int {
	switch k {
	case KindService:
		return 10
	case KindGroup:
		return 20
	case KindContextProfile:
		return 25
	case KindSecurityPolicy:
		return 30
	case KindRule:
		return 40
	default:
		return 100
	}
}
