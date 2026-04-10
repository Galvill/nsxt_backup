package restore

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gv/nsxt-fw-backup/internal/applog"
	"github.com/gv/nsxt-fw-backup/internal/dfw"
	"github.com/gv/nsxt-fw-backup/internal/nsx"
)

// Action is a planned restore operation for one resource.
type Action int

const (
	ActionCreate Action = iota
	ActionSkip
	ActionUpdate
)

func (a Action) String() string {
	switch a {
	case ActionCreate:
		return "CREATE"
	case ActionSkip:
		return "SKIP"
	case ActionUpdate:
		return "UPDATE"
	default:
		return "UNKNOWN"
	}
}

// Step is one row in the restore plan.
type Step struct {
	Path        string
	Kind        string
	DisplayName string
	Action      Action
	Detail      string
}

func kindLabel(path string) string {
	k, ok := dfw.ClassifyPath(path)
	if !ok {
		return "unknown"
	}
	switch k {
	case dfw.KindService:
		return "service"
	case dfw.KindGroup:
		return "group"
	case dfw.KindContextProfile:
		return "context-profile"
	case dfw.KindSecurityPolicy:
		return "security-policy"
	case dfw.KindRule:
		return "rule"
	default:
		return "unknown"
	}
}

func displayNameFromRaw(raw json.RawMessage) string {
	var m struct {
		DisplayName string `json:"display_name"`
	}
	_ = json.Unmarshal(raw, &m)
	return strings.TrimSpace(m.DisplayName)
}

// BuildPlan compares backup resources to the manager and returns ordered steps.
func BuildPlan(c *nsx.Client, apiPrefix string, resources map[string]json.RawMessage, force bool, log *applog.Logger) ([]Step, error) {
	if log == nil {
		log = applog.Discard()
	}
	order := OrderedResourcePaths(resources)
	log.Infof("restore: comparing %d resources to the manager...", len(order))
	var steps []Step
	for i, path := range order {
		raw, ok := resources[path]
		if !ok {
			continue
		}
		rel := dfw.RelFromCanonical(path)
		log.Debugf("plan GET %s", path)
		respBody, status, err := c.Get(apiPrefix, rel)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", path, err)
		}
		st := Step{
			Path:        path,
			Kind:        kindLabel(path),
			DisplayName: displayNameFromRaw(raw),
		}
		switch {
		case status == 404:
			if strings.TrimSpace(apiPrefix) != "" {
				log.Debugf("plan GET %s (default scope)", path)
				_, rootStatus, rerr := c.Get("", rel)
				if rerr != nil {
					return nil, fmt.Errorf("GET %s (default Policy scope): %w", path, rerr)
				}
				if dfw.ShouldSkipRestoreCreateAtTenant(status, rootStatus) {
					st.Action = ActionSkip
					st.Detail = "exists only at default/root Policy scope (not applied under project)"
					steps = append(steps, st)
					continue
				}
			}
			st.Action = ActionCreate
			st.Detail = "object missing on manager"
		case status >= 200 && status < 300:
			if force {
				st.Action = ActionUpdate
				st.Detail = "replace existing object (--force)"
			} else {
				st.Action = ActionSkip
				st.Detail = "already exists (use --force to overwrite)"
			}
		default:
			if err := nsx.DecodeAPIError(status, respBody); err != nil {
				return nil, fmt.Errorf("GET %s: %w", path, err)
			}
			return nil, fmt.Errorf("GET %s: unexpected status %d", path, status)
		}
		steps = append(steps, st)
		n := i + 1
		if n == 1 || n%40 == 0 || n == len(order) {
			log.Infof("restore: planned %d/%d resources...", n, len(order))
		}
	}
	log.Infof("restore: plan complete (%d steps)", len(steps))
	return steps, nil
}
