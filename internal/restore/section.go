package restore

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gv/nsxt-fw-backup/internal/dfw"
)

// FilterResourcesForSection keeps only resources needed for DFW security policies whose
// display_name matches section (exact, trimmed). That includes the policies, their rules,
// and the transitive closure of referenced groups, services, and context profiles in the backup.
// If section is empty, resources is returned unchanged.
func FilterResourcesForSection(resources map[string]json.RawMessage, section string) (map[string]json.RawMessage, error) {
	section = strings.TrimSpace(section)
	if section == "" {
		return resources, nil
	}

	included := make(map[string]json.RawMessage)
	policyIDs := make(map[string]struct{})
	for path, raw := range resources {
		k, ok := dfw.ClassifyPath(path)
		if !ok || k != dfw.KindSecurityPolicy {
			continue
		}
		if displayNameFromRaw(raw) != section {
			continue
		}
		id := dfw.SecurityPolicyIDFromPath(path)
		if id == "" {
			return nil, fmt.Errorf("could not parse security policy id for section %q (path %s)", section, path)
		}
		policyIDs[id] = struct{}{}
		included[path] = raw
	}
	if len(policyIDs) == 0 {
		return nil, fmt.Errorf("no security policy in backup with display_name %q", section)
	}

	for path, raw := range resources {
		k, ok := dfw.ClassifyPath(path)
		if !ok || k != dfw.KindRule {
			continue
		}
		pid := dfw.SecurityPolicyIDFromPath(path)
		if _, want := policyIDs[pid]; want {
			included[path] = raw
		}
	}

	queue := make([]string, 0, len(included))
	for p := range included {
		queue = append(queue, p)
	}

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		raw := included[p]
		for _, ref := range dfw.ExtractInfraPaths(raw) {
			ref = dfw.NormalizeResourceKey(ref)
			if ref == "" {
				continue
			}
			body, ok := resources[ref]
			if !ok {
				continue
			}
			if _, have := included[ref]; have {
				continue
			}
			kind, leaf := dfw.ClassifyPath(ref)
			if !leaf {
				continue
			}
			if kind == dfw.KindSecurityPolicy || kind == dfw.KindRule {
				continue
			}
			included[ref] = body
			queue = append(queue, ref)
		}
	}

	return included, nil
}
