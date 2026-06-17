package github

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Client wraps go-gh clients. raw fetches file contents with the raw Accept
// header (for CODEOWNERS). gql is used for all data collection queries.
type Client struct {
	raw *api.RESTClient
	gql *api.GraphQLClient
}

// NewClient builds a Client using the same auth as `gh` (token + host).
func NewClient() (*Client, error) {
	raw, err := api.NewRESTClient(api.ClientOptions{
		Headers: map[string]string{"Accept": "application/vnd.github.v3.raw"},
	})
	if err != nil {
		return nil, fmt.Errorf("create REST client (is `gh auth login` done?): %w", err)
	}
	gql, err := api.NewGraphQLClient(api.ClientOptions{})
	if err != nil {
		return nil, err
	}
	return &Client{raw: raw, gql: gql}, nil
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
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
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
