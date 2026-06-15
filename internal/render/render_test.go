package render

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jackchuka/gh-org-view/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixture() *github.Org {
	parent := "core"
	return &github.Org{
		Org:         "acme",
		CollectedAt: "2026-06-15T00:00:00Z",
		Teams: []github.Team{
			{Slug: "core", Name: "Core", Description: "Core team", Parent: nil,
				Members: []github.Member{{Login: "alice", Role: "maintainer"}},
				Repos:   []github.Repo{{Name: "acme/api", Permission: "admin"}}},
			{Slug: "web", Name: "Web", Description: "", Parent: &parent,
				Members: []github.Member{}, Repos: []github.Repo{}},
		},
	}
}

func TestHTMLContainsOrgAndTree(t *testing.T) {
	out, err := HTML(fixture())
	require.NoError(t, err)
	assert.NotContains(t, out, "__ORG__")
	assert.NotContains(t, out, "__TREE__")
	assert.NotContains(t, out, "__DATA__")
	assert.Contains(t, out, "acme")
	assert.Contains(t, out, `data-id="core"`)
	assert.Contains(t, out, `data-id="web"`) // nested child rendered
	// Embedded JSON present and HTML-safe (no raw </script breakout).
	assert.Contains(t, out, `id="data"`)
	// The embedded JSON must not contain a raw "</" sequence that could break
	// out of its <script> tag. Inspect only the data blob (the template itself
	// legitimately contains </script> closing tags elsewhere).
	assert.NotContains(t, dataBlob(t, out), "</")
}

// dataBlob returns the JSON text embedded in the <script id="data"> element.
func dataBlob(t *testing.T, out string) string {
	t.Helper()
	const open = `id="data">`
	i := strings.Index(out, open)
	require.GreaterOrEqual(t, i, 0)
	rest := out[i+len(open):]
	j := strings.Index(rest, "</script>")
	require.GreaterOrEqual(t, j, 0)
	return rest[:j]
}

func TestDataBlobEscapesAngleBrackets(t *testing.T) {
	org := &github.Org{Org: "acme", Teams: []github.Team{
		{Slug: "x", Name: "</script><b>", Members: []github.Member{}, Repos: []github.Repo{}},
	}}
	out, err := HTML(org)
	require.NoError(t, err)
	blob := dataBlob(t, out)
	// No raw angle brackets survive in the embedded JSON, so it cannot break
	// out of the <script> tag...
	assert.NotContains(t, blob, "<")
	assert.NotContains(t, blob, ">")
	// ...yet the data round-trips back to the original name intact.
	var got github.Org
	require.NoError(t, json.Unmarshal([]byte(blob), &got))
	assert.Equal(t, "</script><b>", got.Teams[0].Name)
}

func TestEscaping(t *testing.T) {
	assert.Equal(t, "a&amp;b&lt;c&gt;d&quot;e", htmlEscape(`a&b<c>d"e`))
}

// Golden: regenerate with UPDATE_GOLDEN=1 go test ./internal/render/
func TestGolden(t *testing.T) {
	out, err := HTML(fixture())
	require.NoError(t, err)
	path := "testdata/golden.html"
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(path, []byte(out), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, strings.TrimRight(string(want), "\n"), strings.TrimRight(out, "\n"))
}
