package restore

import (
	"encoding/json"
	"testing"

	"github.com/gv/nsxt-fw-backup/internal/dfw"
)

func TestTopoSortGroups_linearChain(t *testing.T) {
	resources := map[string]json.RawMessage{
		"/infra/domains/default/groups/a": []byte(`{"path":"/infra/domains/default/groups/a"}`),
		"/infra/domains/default/groups/b": []byte(`{"expression":[{"paths":["/infra/domains/default/groups/a"]}]}`),
		"/infra/domains/default/groups/c": []byte(`{"expression":[{"paths":["/infra/domains/default/groups/b"]}]}`),
	}
	order := topoSortGroups([]string{
		"/infra/domains/default/groups/c",
		"/infra/domains/default/groups/a",
		"/infra/domains/default/groups/b",
	}, resources)
	// a before b before c
	idx := func(p string) int {
		for i, x := range order {
			if x == p {
				return i
			}
		}
		return -1
	}
	if idx("/infra/domains/default/groups/a") >= idx("/infra/domains/default/groups/b") {
		t.Fatalf("order %v: a should precede b", order)
	}
	if idx("/infra/domains/default/groups/b") >= idx("/infra/domains/default/groups/c") {
		t.Fatalf("order %v: b should precede c", order)
	}
}

func TestOrderedResourcePaths_priority(t *testing.T) {
	resources := map[string]json.RawMessage{
		"/infra/domains/default/security-policies/p1/rules/r1": []byte(`{}`),
		"/infra/domains/default/security-policies/p1":          []byte(`{}`),
		"/infra/services/svc1":                                 []byte(`{}`),
		"/infra/domains/default/groups/g1":                     []byte(`{}`),
	}
	order := OrderedResourcePaths(resources)
	if len(order) != 4 {
		t.Fatalf("len %d", len(order))
	}
	if order[0] != "/infra/services/svc1" {
		t.Fatalf("first should be service, got %v", order)
	}
	if dfw.KindPriority(dfw.KindRule) <= dfw.KindPriority(dfw.KindSecurityPolicy) {
		t.Fatal("rule should sort after policy")
	}
	// service, group, policy, rule
	if order[1] != "/infra/domains/default/groups/g1" {
		t.Fatalf("second should be group: %v", order)
	}
	if order[2] != "/infra/domains/default/security-policies/p1" {
		t.Fatalf("third should be policy: %v", order)
	}
	if order[3] != "/infra/domains/default/security-policies/p1/rules/r1" {
		t.Fatalf("fourth should be rule: %v", order)
	}
}
