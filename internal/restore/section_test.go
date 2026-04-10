package restore

import (
	"encoding/json"
	"testing"
)

func TestFilterResourcesForSection(t *testing.T) {
	resources := map[string]json.RawMessage{
		"/infra/domains/default/security-policies/keep":          []byte(`{"display_name":"Section A","id":"keep"}`),
		"/infra/domains/default/security-policies/drop":          []byte(`{"display_name":"Other","id":"drop"}`),
		"/infra/domains/default/security-policies/keep/rules/r1": []byte(`{"display_name":"r1","services":["/infra/services/svc1"]}`),
		"/infra/services/svc1":                                   []byte(`{"display_name":"svc1"}`),
		"/infra/domains/default/groups/g1":                       []byte(`{"display_name":"g1","expression":[{"paths":["/infra/domains/default/groups/g2"]}]}`),
		"/infra/domains/default/groups/g2":                       []byte(`{"display_name":"g2"}`),
		"/infra/domains/default/security-policies/keep/rules/r2": []byte(`{"source_groups":["/infra/domains/default/groups/g1"]}`),
	}

	out, err := FilterResourcesForSection(resources, "Section A")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 6 {
		t.Fatalf("want 6 resources (policy, 2 rules, svc, 2 groups), got %d: %v", len(out), keys(out))
	}
	for _, k := range []string{
		"/infra/domains/default/security-policies/keep",
		"/infra/domains/default/security-policies/keep/rules/r1",
		"/infra/domains/default/security-policies/keep/rules/r2",
		"/infra/services/svc1",
		"/infra/domains/default/groups/g1",
		"/infra/domains/default/groups/g2",
	} {
		if _, ok := out[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	if _, ok := out["/infra/domains/default/security-policies/drop"]; ok {
		t.Fatal("should not include other policy")
	}
}

func TestFilterResourcesForSection_emptyMeansAll(t *testing.T) {
	m := map[string]json.RawMessage{"/infra/services/x": []byte(`{}`)}
	out, err := FilterResourcesForSection(m, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out["/infra/services/x"] == nil {
		t.Fatalf("expected same map contents, got %#v", out)
	}
}

func TestFilterResourcesForSection_notFound(t *testing.T) {
	_, err := FilterResourcesForSection(map[string]json.RawMessage{
		"/infra/domains/default/security-policies/p": []byte(`{"display_name":"X"}`),
	}, "missing")
	if err == nil {
		t.Fatal("expected error")
	}
}

func keys(m map[string]json.RawMessage) []string {
	var s []string
	for k := range m {
		s = append(s, k)
	}
	return s
}
