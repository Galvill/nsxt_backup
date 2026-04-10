package nsx

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// AuthFromEnv builds a RoundTripper that adds credentials from environment variables only.
// Basic: NSXT_USERNAME + NSXT_PASSWORD (and host from flags/env).
// Bearer: NSXT_BEARER_TOKEN or NSXT_API_KEY (either name accepted).
func AuthFromEnv() (func(http.Header) error, error) {
	user := os.Getenv("NSXT_USERNAME")
	pass := os.Getenv("NSXT_PASSWORD")
	bearer := strings.TrimSpace(os.Getenv("NSXT_BEARER_TOKEN"))
	if bearer == "" {
		bearer = strings.TrimSpace(os.Getenv("NSXT_API_KEY"))
	}

	if bearer != "" {
		if user != "" || pass != "" {
			return nil, fmt.Errorf("set either bearer token (NSXT_BEARER_TOKEN or NSXT_API_KEY) or basic auth (NSXT_USERNAME/NSXT_PASSWORD), not both")
		}
		return func(h http.Header) error {
			h.Set("Authorization", "Bearer "+bearer)
			return nil
		}, nil
	}

	if user == "" || pass == "" {
		return nil, fmt.Errorf("missing credentials: set NSXT_USERNAME and NSXT_PASSWORD, or NSXT_BEARER_TOKEN / NSXT_API_KEY")
	}

	return func(h http.Header) error {
		h.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
		return nil
	}, nil
}
