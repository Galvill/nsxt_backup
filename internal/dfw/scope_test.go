package dfw

import "testing"

func TestDecideFetchReferencedLeafForBackup(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		tenant, root int
		wantFetch    bool
		wantErr      bool
	}{
		{"tenant 200", 200, 0, true, false},
		{"tenant 201", 201, 0, true, false},
		{"tenant 404 root 200", 404, 200, false, false},
		{"tenant 404 root 404", 404, 404, false, true},
		{"tenant 404 root 500", 404, 500, false, true},
		{"tenant 403", 403, 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fetch, err := DecideFetchReferencedLeafForBackup(tt.tenant, tt.root)
			if fetch != tt.wantFetch {
				t.Errorf("fetch=%v want %v", fetch, tt.wantFetch)
			}
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
		})
	}
}

func TestShouldSkipRestoreCreateAtTenant(t *testing.T) {
	t.Parallel()
	if !ShouldSkipRestoreCreateAtTenant(404, 200) {
		t.Fatal("404+200 should skip")
	}
	if ShouldSkipRestoreCreateAtTenant(404, 404) {
		t.Fatal("404+404 should not skip")
	}
	if ShouldSkipRestoreCreateAtTenant(200, 200) {
		t.Fatal("200+200 should not skip")
	}
}
