package dfw

import (
	"encoding/json"
)

// stripJSONFields returns a copy of JSON object with given top-level keys removed.
func stripJSONFields(raw []byte, keys ...string) []byte {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	for _, k := range keys {
		delete(m, k)
	}
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}
