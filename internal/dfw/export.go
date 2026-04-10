package dfw

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gv/nsxt-fw-backup/internal/applog"
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
	Log         *applog.Logger
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
	log := opts.Log
	if log == nil {
		log = applog.Discard()
	}

	scopeLabel := "default Policy scope"
	if strings.TrimSpace(opts.APIPrefix) != "" {
		scopeLabel = opts.APIPrefix
	}
	log.Infof("export: domain %q, scope %s", domain, scopeLabel)
	if strings.TrimSpace(opts.Section) != "" {
		log.Infof("export: filtering to security policy display_name %q", strings.TrimSpace(opts.Section))
	}
	log.Infof("export: listing security policies...")

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
	log.Infof("export: %d security %s to back up (of %d listed in domain)",
		len(policies), pluralPolicies(len(policies)), len(summaries))
	log.Infof("export: walking policies, rules, and referenced objects...")

	seen := make(map[string]struct{})
	queue := make([]string, 0, 64)
	prefetched := make(map[string][]byte)
	tenantPrefix := strings.TrimSpace(opts.APIPrefix)

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
		var body []byte
		var status int
		var err error
		if pb, ok := prefetched[path]; ok {
			body = pb
			status = 200
			delete(prefetched, path)
		} else {
			log.Debugf("GET %s", path)
			body, status, err = opts.Client.Get(opts.APIPrefix, rel)
			if err != nil {
				return nil, err
			}
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
		n := len(doc.Resources)
		if n == 1 || n%40 == 0 {
			log.Infof("export: collected %d resources...", n)
		}

		for _, ref := range ExtractInfraPaths(body) {
			ref = NormalizeResourceKey(ref)
			if ref == "" || ref == path {
				continue
			}
			if _, done := seen[ref]; done {
				continue
			}
			if _, ok := ClassifyPath(ref); ok {
				if tenantPrefix != "" {
					if _, have := prefetched[ref]; !have {
						subRel := RelFromCanonical(ref)
						fetch, pre, perr := ResolveReferencedLeafForTenantBackup(opts.Client, tenantPrefix, subRel)
						if perr != nil {
							return nil, fmt.Errorf("%w (ref %s)", perr, ref)
						}
						if !fetch {
							continue
						}
						if len(pre) > 0 {
							prefetched[ref] = append([]byte(nil), pre...)
						}
					}
				}
				queue = append(queue, ref)
			}
		}

		if kind == KindSecurityPolicy {
			policyID := SecurityPolicyIDFromPath(path)
			if policyID == "" {
				continue
			}
			ruleList := fmt.Sprintf("infra/domains/%s/security-policies/%s/rules", domainPathSeg(domain), policyID)
			log.Debugf("LIST rules %s", ruleList)
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

	log.Infof("export: done (%d resources)", len(doc.Resources))
	return doc, nil
}

func pluralPolicies(n int) string {
	if n == 1 {
		return "policy"
	}
	return "policies"
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
