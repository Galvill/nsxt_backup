package backup

import (
	"net/url"
	"strings"
)

// RecordedAPIPrefix returns the multi-tenant Policy API path prefix implied by the backup scope,
// preferring api_prefix and falling back to org + project when both are set.
func (s Scope) RecordedAPIPrefix() string {
	p := strings.TrimSpace(s.APIPrefix)
	if p != "" {
		return p
	}
	o := strings.TrimSpace(s.Org)
	pr := strings.TrimSpace(s.Project)
	if o != "" && pr != "" {
		return "orgs/" + url.PathEscape(o) + "/projects/" + url.PathEscape(pr)
	}
	return ""
}
