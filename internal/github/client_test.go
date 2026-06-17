package github

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/require"
)

// stubRT serves canned REST bodies keyed by URL path (plus query), ignoring
// host, for the raw CODEOWNERS client. Unknown paths return 404. The
// multi-body / Link-header logic is vestigial — pagination is no longer used.
type stubRT struct {
	pages map[string][]string // path -> ordered page bodies
	calls map[string]int
}

func (s *stubRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	// Use EscapedPath so the key reflects the bytes actually sent on the wire
	// (e.g. a "/" escaped to %2F would NOT match an unescaped key) — this is
	// what GitHub's API sees, so over-escaped paths correctly 404 here.
	p := req.URL.EscapedPath()
	if req.URL.RawQuery != "" {
		p += "?" + req.URL.RawQuery
	}
	s.calls[p]++
	idx := s.calls[p] - 1
	bodies, ok := s.pages[p]
	if !ok {
		return &http.Response{StatusCode: 404, Header: http.Header{},
			Body: io.NopCloser(strings.NewReader("Not Found")), Request: req}, nil
	}
	body := "[]"
	if idx < len(bodies) {
		body = bodies[idx]
	}
	h := http.Header{}
	if idx+1 < len(bodies) {
		// Emit the request's own absolute URL as the next link so the stub
		// advances by call-count for the same key on subsequent requests.
		h.Set("Link", "<"+req.URL.String()+">; rel=\"next\"")
	}
	return &http.Response{
		StatusCode: 200,
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func testClient(t *testing.T, rt http.RoundTripper) *Client {
	t.Helper()
	opts := api.ClientOptions{Host: "github.com", AuthToken: "test", Transport: rt}
	rest, err := api.NewRESTClient(opts)
	require.NoError(t, err)
	gql, err := api.NewGraphQLClient(opts)
	require.NoError(t, err)
	return &Client{raw: rest, gql: gql}
}
