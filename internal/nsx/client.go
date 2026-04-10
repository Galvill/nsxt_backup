package nsx

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client calls NSX-T Policy API (local manager).
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
	setAuth    func(http.Header) error
}

// Options for NewClient.
type Options struct {
	Host               string // hostname or https://host[:port]
	InsecureSkipVerify bool
}

// NewClient builds a Policy API client. Base path is always /policy/api/v1.
func NewClient(opts Options, setAuth func(http.Header) error) (*Client, error) {
	if setAuth == nil {
		return nil, fmt.Errorf("auth is required")
	}
	host := strings.TrimSpace(opts.Host)
	if host == "" {
		host = strings.TrimSpace(os.Getenv("NSXT_MANAGER_HOST"))
	}
	if host == "" {
		host = strings.TrimSpace(os.Getenv("NSXT_HOST"))
	}
	if host == "" {
		return nil, fmt.Errorf("manager host is required (flag --host or NSXT_MANAGER_HOST / NSXT_HOST)")
	}

	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("invalid host URL: %w", err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	if u.Path != "" && u.Path != "/" {
		return nil, fmt.Errorf("host must be scheme://host[:port] without path; got path %q", u.Path)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.InsecureSkipVerify, //nolint:gosec // optional lab use via env/flag
		},
	}

	return &Client{
		baseURL: u,
		httpClient: &http.Client{
			Timeout:   120 * time.Second,
			Transport: tr,
		},
		setAuth: setAuth,
	}, nil
}

func (c *Client) policyV1Base(prefix string) string {
	// https://host/policy/api/v1[/orgs/.../projects/...]
	p := c.baseURL.String() + "/policy/api/v1"
	if prefix != "" {
		p += "/" + prefix
	}
	return p
}

func (c *Client) joinURL(prefix, rel string) string {
	rel = strings.TrimPrefix(rel, "/")
	base := c.policyV1Base(prefix)
	return base + "/" + rel
}

// Get performs GET and returns body and status.
func (c *Client) Get(prefix, path string) (body []byte, status int, err error) {
	return c.do(prefix, http.MethodGet, path, nil)
}

// Put performs PUT with JSON body.
func (c *Client) Put(prefix, path string, jsonBody []byte) (body []byte, status int, err error) {
	return c.do(prefix, http.MethodPut, path, jsonBody)
}

func (c *Client) do(prefix, method, path string, jsonBody []byte) ([]byte, int, error) {
	u := c.joinURL(prefix, path)
	var rdr io.Reader
	if jsonBody != nil {
		rdr = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequest(method, u, rdr)
	if err != nil {
		return nil, 0, err
	}
	if err := c.setAuth(req.Header); err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return b, resp.StatusCode, nil
}

// APIError is a decoded Policy API error payload when possible.
type APIError struct {
	StatusCode int
	Body       string
	HTTPError  string
	Message    string
	Details    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("nsx api %d: %s", e.StatusCode, e.Message)
	}
	if e.HTTPError != "" {
		return fmt.Sprintf("nsx api %d: %s", e.StatusCode, e.HTTPError)
	}
	return fmt.Sprintf("nsx api %d: %s", e.StatusCode, strings.TrimSpace(e.Body))
}

// DecodeAPIError tries to parse standard error JSON.
func DecodeAPIError(status int, body []byte) error {
	if status < 400 {
		return nil
	}
	ae := &APIError{StatusCode: status, Body: string(body)}
	var wrap struct {
		ErrorMessage  string `json:"error_message"`
		ErrorCode     int    `json:"error_code"`
		ModuleName    string `json:"module_name"`
		RelatedErrors []struct {
			ErrorMessage string `json:"error_message"`
		} `json:"related_errors"`
	}
	if json.Unmarshal(body, &wrap) == nil && wrap.ErrorMessage != "" {
		ae.Message = wrap.ErrorMessage
		if wrap.ModuleName != "" {
			ae.Details = wrap.ModuleName
		}
	} else {
		ae.HTTPError = http.StatusText(status)
	}
	return ae
}
