package dfw

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/gv/nsxt-fw-backup/internal/nsx"
)

type listEnvelope struct {
	Results []json.RawMessage `json:"results"`
	Cursor  string            `json:"cursor"`
}

// CollectListResults pages through a Policy list endpoint and returns all result objects.
func CollectListResults(c *nsx.Client, apiPrefix, listPath string) ([]json.RawMessage, error) {
	listPath = strings.TrimPrefix(listPath, "/")
	var all []json.RawMessage
	cursor := ""
	for i := 0; i < 1000; i++ {
		p := listPath
		q := url.Values{}
		q.Set("page_size", "1000")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		if strings.Contains(listPath, "?") {
			p = listPath + "&" + q.Encode()
		} else {
			p = listPath + "?" + q.Encode()
		}
		body, status, err := c.Get(apiPrefix, p)
		if err != nil {
			return nil, err
		}
		if status == 404 {
			return nil, fmt.Errorf("list not found: %s (%d)", listPath, status)
		}
		if err := nsx.DecodeAPIError(status, body); err != nil {
			return nil, err
		}
		var env listEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("decode list %s: %w", listPath, err)
		}
		all = append(all, env.Results...)
		if env.Cursor == "" {
			break
		}
		cursor = env.Cursor
	}
	return all, nil
}
