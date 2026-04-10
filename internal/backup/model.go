package backup

import "encoding/json"

const FormatVersion = 1

// Document is the on-disk backup envelope.
type Document struct {
	FormatVersion int                        `json:"format_version"`
	CreatedAt     string                     `json:"created_at"`
	ManagerHost   string                     `json:"manager_host,omitempty"`
	Scope         Scope                      `json:"scope"`
	Resources     map[string]json.RawMessage `json:"resources"`
}

// Scope describes what was exported.
type Scope struct {
	Domain    string `json:"domain"`
	Section   string `json:"section,omitempty"`
	Org       string `json:"org,omitempty"`
	Project   string `json:"project,omitempty"`
	APIPrefix string `json:"api_prefix,omitempty"`
}
