package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Client wraps go-gh REST clients. rest decodes JSON; raw fetches file contents
// with the raw Accept header (for CODEOWNERS).
type Client struct {
	rest *api.RESTClient
	raw  *api.RESTClient
}

// NewClient builds a Client using the same auth as `gh` (token + host).
func NewClient() (*Client, error) {
	rest, err := api.NewRESTClient(api.ClientOptions{})
	if err != nil {
		return nil, fmt.Errorf("create REST client (is `gh auth login` done?): %w", err)
	}
	raw, err := api.NewRESTClient(api.ClientOptions{
		Headers: map[string]string{"Accept": "application/vnd.github.v3.raw"},
	})
	if err != nil {
		return nil, err
	}
	return &Client{rest: rest, raw: raw}, nil
}

// paginate GETs every page of a list endpoint, following Link rel="next".
func paginate[T any](c *Client, path string) ([]T, error) {
	var all []T
	for path != "" {
		resp, err := c.rest.Request(http.MethodGet, path, nil)
		if err != nil {
			return nil, mapAuthError(err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var page []T
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		all = append(all, page...)
		next := parseNextLink(resp.Header.Get("Link"))
		// Convert absolute URL to a relative path (path only, no query) so
		// go-gh re-adds the host. The stub uses call-count to serve pages.
		if next != "" {
			if u, err := url.Parse(next); err == nil && u.IsAbs() {
				next = strings.TrimPrefix(u.Path, "/")
			}
		}
		path = next
	}
	return all, nil
}

// getRaw returns the raw bytes of a repo content path, or (nil,false) on 404.
func (c *Client) getRaw(path string) ([]byte, bool, error) {
	resp, err := c.raw.Request(http.MethodGet, path, nil)
	if err != nil {
		var he *api.HTTPError
		if errors.As(err, &he) && he.StatusCode == http.StatusNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// parseNextLink extracts the rel="next" URL from a Link header, or "".
func parseNextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		segs := strings.Split(part, ";")
		if len(segs) < 2 {
			continue
		}
		isNext := false
		for _, s := range segs[1:] {
			if strings.Contains(s, `rel="next"`) {
				isNext = true
			}
		}
		if !isNext {
			continue
		}
		url := strings.TrimSpace(segs[0])
		url = strings.TrimPrefix(url, "<")
		url = strings.TrimSuffix(url, ">")
		return url
	}
	return ""
}

// mapAuthError turns 401/403 into an actionable scope message.
func mapAuthError(err error) error {
	var he *api.HTTPError
	if errors.As(err, &he) {
		switch he.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("not authenticated — run `gh auth login`: %w", err)
		case http.StatusForbidden:
			return fmt.Errorf("forbidden — you may need the read:org scope; run `gh auth refresh -s read:org`: %w", err)
		}
	}
	return err
}
