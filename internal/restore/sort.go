package restore

import (
	"encoding/json"
	"sort"

	"github.com/gv/nsxt-fw-backup/internal/dfw"
)

// OrderedResourcePaths returns restore order: services, groups (topo), context-profiles, policies, rules.
func OrderedResourcePaths(resources map[string]json.RawMessage) []string {
	var services, groups, profiles, policies, rules []string
	for p := range resources {
		k, ok := dfw.ClassifyPath(p)
		if !ok {
			continue
		}
		switch k {
		case dfw.KindService:
			services = append(services, p)
		case dfw.KindGroup:
			groups = append(groups, p)
		case dfw.KindContextProfile:
			profiles = append(profiles, p)
		case dfw.KindSecurityPolicy:
			policies = append(policies, p)
		case dfw.KindRule:
			rules = append(rules, p)
		}
	}
	sort.Strings(services)
	sort.Strings(profiles)
	sort.Strings(policies)
	sort.Strings(rules)

	orderedGroups := topoSortGroups(groups, resources)

	out := make([]string, 0, len(resources))
	out = append(out, services...)
	out = append(out, orderedGroups...)
	out = append(out, profiles...)
	out = append(out, policies...)
	out = append(out, rules...)

	used := make(map[string]struct{}, len(out))
	for _, p := range out {
		used[p] = struct{}{}
	}
	var rest []string
	for p := range resources {
		if _, ok := used[p]; !ok {
			rest = append(rest, p)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

func topoSortGroups(groups []string, resources map[string]json.RawMessage) []string {
	set := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		set[g] = struct{}{}
	}

	indeg := make(map[string]int)
	adj := make(map[string][]string)
	for _, g := range groups {
		indeg[g] = 0
	}
	for _, g := range groups {
		raw := resources[g]
		for _, ref := range dfw.ExtractInfraPaths(raw) {
			ref = dfw.NormalizeResourceKey(ref)
			if ref == g {
				continue
			}
			if _, ok := set[ref]; !ok {
				continue
			}
			adj[ref] = append(adj[ref], g)
			indeg[g]++
		}
	}

	var q []string
	for _, g := range groups {
		if indeg[g] == 0 {
			q = append(q, g)
		}
	}
	sort.Strings(q)

	var order []string
	for len(q) > 0 {
		n := q[0]
		q = q[1:]
		order = append(order, n)
		for _, m := range adj[n] {
			indeg[m]--
			if indeg[m] == 0 {
				q = append(q, m)
			}
		}
		sort.Strings(q)
	}

	if len(order) != len(groups) {
		sort.Strings(groups)
		return groups
	}
	return order
}
