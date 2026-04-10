package restore

import (
	"encoding/json"
	"fmt"

	"github.com/gv/nsxt-fw-backup/internal/applog"
	"github.com/gv/nsxt-fw-backup/internal/dfw"
	"github.com/gv/nsxt-fw-backup/internal/nsx"
)

// Apply executes CREATE and UPDATE steps using PUT with backup bodies.
func Apply(c *nsx.Client, apiPrefix string, resources map[string]json.RawMessage, steps []Step, log *applog.Logger) error {
	if log == nil {
		log = applog.Discard()
	}
	var toRun int
	for _, st := range steps {
		if st.Action != ActionSkip {
			toRun++
		}
	}
	log.Infof("restore: applying %d create/update operation(s)...", toRun)

	done := 0
	for _, st := range steps {
		if st.Action == ActionSkip {
			continue
		}
		raw, ok := resources[st.Path]
		if !ok {
			return fmt.Errorf("missing body for %s", st.Path)
		}
		rel := dfw.RelFromCanonical(st.Path)
		log.Debugf("PUT %s (%s)", st.Path, st.Action.String())
		body, status, err := c.Put(apiPrefix, rel, stripReadOnlyFields(raw))
		if err != nil {
			return fmt.Errorf("PUT %s: %w", st.Path, err)
		}
		if err := nsx.DecodeAPIError(status, body); err != nil {
			return fmt.Errorf("PUT %s: %w", st.Path, err)
		}
		done++
		if done == 1 || done%20 == 0 || done == toRun {
			log.Infof("restore: applied %d/%d operation(s)...", done, toRun)
		}
	}
	log.Infof("restore: apply finished")
	return nil
}

// stripReadOnlyFields removes fields that commonly cause PUT failures on create.
func stripReadOnlyFields(raw []byte) []byte {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	delete(m, "_create_time")
	delete(m, "_create_user")
	delete(m, "_last_modified_time")
	delete(m, "_last_modified_user")
	delete(m, "_system_owned")
	delete(m, "marked_for_delete")
	// revision is often read-only on PUT for some resources; NSX may expect If-Match instead
	delete(m, "_revision")
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}
