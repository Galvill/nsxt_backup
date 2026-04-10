package dfw

import "testing"

func TestSecurityPolicyIDFromPath(t *testing.T) {
	cases := map[string]string{
		"/infra/domains/default/security-policies/p1":                   "p1",
		"/infra/domains/default/security-policies/p1/rules/r1":          "p1",
		"/orgs/x/projects/y/infra/domains/default/security-policies/p2": "p2",
		"/infra/services/foo": "",
	}
	for path, want := range cases {
		if got := SecurityPolicyIDFromPath(path); got != want {
			t.Errorf("SecurityPolicyIDFromPath(%q) = %q, want %q", path, got, want)
		}
	}
}
