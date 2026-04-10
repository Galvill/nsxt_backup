package dfw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gv/nsxt-fw-backup/internal/backup"
	"github.com/gv/nsxt-fw-backup/internal/nsx"
)

// PolicySummary is a minimal security policy list entry.
type PolicySummary struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Path        string `json:"path"`
}

// ExportOptions configures a DFW export run.
type ExportOptions struct {
	Client      *nsx.Client
	APIPrefix   string
	Domain      string
	Section     string
	RedactHost  bool
	ManagerHost string
	Org         string
	Project     string
}

// Export downloads security policies (optional section filter), rules, and referenced infra objects.
func Export(ctx context.Context, opts ExportOptions) (*backup.Document, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("client is required")
	}
	domain := strings.TrimSpace(opts.Domain)
	if domain == "" {
		domain = "default"
	}

	doc := &backup.Document{
		FormatVersion: backup.FormatVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Scope: backup.Scope{
			Domain:    domain,
			Section:   strings.TrimSpace(opts.Section),
			APIPrefix: opts.APIPrefix,
			Org:       strings.TrimSpace(opts.Org),
			Project:   strings.TrimSpace(opts.Project),
		},
		Resources: make(map[string]json.RawMessage),
	}
	if !opts.RedactHost && opts.ManagerHost != "" {
		doc.ManagerHost = opts.ManagerHost
	}

	listPath := fmt.Sprintf("infra/domains/%s/security-policies", domainPathSeg(domain))
	summaries, err := CollectListResults(opts.Client, opts.APIPrefix, listPath)
	if err != nil {
		return nil, err
	}

	var policies []PolicySummary
	for _, raw := range summaries {
		var s PolicySummary
		if json.Unmarshal(raw, &s) != nil || s.ID == "" {
			continue
		}
		if opts.Section != "" && s.DisplayName != opts.Section {
			continue
		}
		policies = append(policies, s)
	}
	if opts.Section != "" && len(policies) == 0 {
		return nil, fmt.Errorf("no security policy found with display_name %q", opts.Section)
	}

	seen := make(map[string]struct{})
	queue := make([]string, 0, 64)

	for _, p := range policies {
		policyPath := policyCanonicalPath(domain, p)
		queue = append(queue, policyPath)
	}

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path := NormalizeResourceKey(queue[0])
		queue = queue[1:]
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		kind, leaf := ClassifyPath(path)
		if !leaf {
			continue
		}

		rel := RelFromCanonical(path)
		body, status, err := opts.Client.Get(opts.APIPrefix, rel)
		if err != nil {
			return nil, err
		}
		if status == 404 {
			return nil, fmt.Errorf("GET %s: not found (404)", path)
		}
		if err := nsx.DecodeAPIError(status, body); err != nil {
			return nil, fmt.Errorf("%w (path %s)", err, path)
		}
		storeBody := body
		if kind == KindSecurityPolicy {
			storeBody = stripJSONFields(body, "rules")
		}
		doc.Resources[path] = json.RawMessage(append([]byte(nil), storeBody...))

		for _, ref := range ExtractInfraPaths(body) {
			ref = NormalizeResourceKey(ref)
			if ref == "" || ref == path {
				continue
			}
			if _, done := seen[ref]; done {
				continue
			}
			if _, ok := ClassifyPath(ref); ok {
				queue = append(queue, ref)
			}
		}

		if kind == KindSecurityPolicy {
			policyID := segmentAfter(path, "security-policies")
			if policyID == "" {
				continue
			}
			ruleList := fmt.Sprintf("infra/domains/%s/security-policies/%s/rules", domainPathSeg(domain), policyID)
			ruleObjs, err := CollectListResults(opts.Client, opts.APIPrefix, ruleList)
			if err != nil {
				return nil, fmt.Errorf("list rules for %s: %w", path, err)
			}
			for _, rr := range ruleObjs {
				var meta struct {
					ID   string `json:"id"`
					Path string `json:"path"`
				}
				if json.Unmarshal(rr, &meta) != nil || meta.ID == "" {
					continue
				}
				rulePath := NormalizeResourceKey(meta.Path)
				if rulePath == "" {
					rulePath = normalizePath(fmt.Sprintf("/infra/domains/%s/security-policies/%s/rules/%s",
						domainPathSeg(domain), policyID, meta.ID))
				}
				if _, done := seen[rulePath]; !done {
					queue = append(queue, rulePath)
				}
			}
		}
	}

	return doc, nil
}

func domainPathSeg(domain string) string {
	return strings.Trim(domain, "/")
}

func policyCanonicalPath(domain string, p PolicySummary) string {
	if p.Path != "" {
		return NormalizeResourceKey(p.Path)
	}
	return normalizePath(fmt.Sprintf("/infra/domains/%s/security-policies/%s", domainPathSeg(domain), p.ID))
}

func segmentAfter(path, marker string) string {
	path = NormalizeResourceKey(path)
	needle := "/" + marker + "/"
	i := strings.Index(path, needle)
	if i < 0 {
		return ""
	}
	rest := path[i+len(needle):]
	j := strings.Index(rest, "/")
	if j < 0 {
		return rest
	}
	return rest[:j]
}
