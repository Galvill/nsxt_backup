package backup

import "testing"

func TestScope_RecordedAPIPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    Scope
		want string
	}{
		{"empty", Scope{}, ""},
		{"api_prefix only", Scope{APIPrefix: "orgs/a/projects/b"}, "orgs/a/projects/b"},
		{"api_prefix wins over org", Scope{APIPrefix: "orgs/x/projects/y", Org: "a", Project: "b"}, "orgs/x/projects/y"},
		{"org+project fallback", Scope{Org: "my org", Project: "p1"}, "orgs/my%20org/projects/p1"},
		{"partial org only", Scope{Org: "a"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.s.RecordedAPIPrefix(); got != tt.want {
				t.Errorf("RecordedAPIPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
