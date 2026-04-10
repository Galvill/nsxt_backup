package dfw

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestExtractInfraPaths(t *testing.T) {
	raw := []byte(`{
		"source_groups": ["/infra/domains/default/groups/a"],
		"destination_groups": ["/infra/domains/default/groups/b"],
		"services": ["/infra/services/ICMP-ALL"],
		"notes": "see /infra/domains/default/context-profiles/p1 for profile"
	}`)
	got := ExtractInfraPaths(raw)
	sort.Strings(got)
	want := []string{
		"/infra/domains/default/context-profiles/p1",
		"/infra/domains/default/groups/a",
		"/infra/domains/default/groups/b",
		"/infra/services/ICMP-ALL",
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("got %d paths, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestClassifyPath(t *testing.T) {
	cases := []struct {
		path string
		kind ResourceKind
		ok   bool
	}{
		{"/infra/services/foo", KindService, true},
		{"/infra/domains/default/groups/g1", KindGroup, true},
		{"/infra/domains/default/context-profiles/c1", KindContextProfile, true},
		{"/infra/domains/default/security-policies/p1", KindSecurityPolicy, true},
		{"/infra/domains/default/security-policies/p1/rules/r1", KindRule, true},
		{"/infra/domains/default/security-policies", KindUnknown, false},
		{"/other", KindUnknown, false},
	}
	for _, tc := range cases {
		k, ok := ClassifyPath(tc.path)
		if ok != tc.ok || k != tc.kind {
			t.Errorf("ClassifyPath(%q) = (%v,%v), want (%v,%v)", tc.path, k, ok, tc.kind, tc.ok)
		}
	}
}

func TestNormalizeResourceKey(t *testing.T) {
	in := "/orgs/default/projects/acme/infra/domains/default/groups/g1"
	want := "/infra/domains/default/groups/g1"
	if got := NormalizeResourceKey(in); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripJSONFields(t *testing.T) {
	in := []byte(`{"id":"p1","rules":[{"id":"r1"}],"display_name":"P"}`)
	out := stripJSONFields(in, "rules")
	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["rules"]; ok {
		t.Fatal("rules should be stripped")
	}
}
