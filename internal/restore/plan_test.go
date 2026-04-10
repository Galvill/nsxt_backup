package restore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gv/nsxt-fw-backup/internal/nsx"
)

func TestBuildPlan_skipWhenExists(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/policy/api/v1/infra/services/existing", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"display_name":"Existing"}`))
	})
	mux.HandleFunc("/policy/api/v1/infra/services/missing", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "https://")
	c, err := nsx.NewClient(nsx.Options{Host: host, InsecureSkipVerify: true}, func(h http.Header) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	resources := map[string]json.RawMessage{
		"/infra/services/existing": []byte(`{"display_name":"Existing"}`),
		"/infra/services/missing":  []byte(`{"display_name":"Missing"}`),
	}
	steps, err := BuildPlan(c, "", resources, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 {
		t.Fatalf("steps: %d", len(steps))
	}
	// Ordered: existing (service name sort) then missing? services sort: existing before missing
	if steps[0].Path != "/infra/services/existing" {
		t.Fatalf("order: %#v", steps)
	}
	if steps[0].Action != ActionSkip {
		t.Fatalf("existing: %v", steps[0].Action)
	}
	if steps[1].Action != ActionCreate {
		t.Fatalf("missing: %v", steps[1].Action)
	}
}

func TestActionString(t *testing.T) {
	if ActionCreate.String() != "CREATE" {
		t.Fatal()
	}
}
