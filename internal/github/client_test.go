package github

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNextLink(t *testing.T) {
	hdr := `<https://api.github.com/x?page=2>; rel="next", <https://api.github.com/x?page=5>; rel="last"`
	assert.Equal(t, "https://api.github.com/x?page=2", parseNextLink(hdr))
	assert.Equal(t, "", parseNextLink(`<https://api.github.com/x?page=5>; rel="last"`))
	assert.Equal(t, "", parseNextLink(""))
}

// stubRT serves canned bodies keyed by URL path (plus query), ignoring host. It
// sets a Link: next header on the first call to a path then drops it, to
// exercise pagination. Unknown paths return 404 (so raw fetches behave).
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
		// Emit the request's own absolute URL as the next link. paginate feeds
		// it straight back (go-gh passes absolute URLs through), so the stub
		// serves the next body by call-count for the same key.
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
	return &Client{rest: rest, raw: rest, gql: gql}
}

func TestPaginateConcatenatesPages(t *testing.T) {
	rt := &stubRT{pages: map[string][]string{
		"/orgs/acme/teams": {`[{"slug":"a"}]`, `[{"slug":"b"}]`},
	}}
	c := testClient(t, rt)
	got, err := paginate[Team](c, "orgs/acme/teams")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].Slug)
	assert.Equal(t, "b", got[1].Slug)
}
