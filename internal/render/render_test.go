package render

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestSidebarShowsChildMemberCount(t *testing.T) {
	parent := "core"
	org := &github.Org{Org: "acme", Teams: []github.Team{
		{Slug: "core", Name: "Core", Members: []github.Member{{Login: "alice", Role: "maintainer"}}, Repos: []github.Repo{}},
		{Slug: "web", Name: "Web", Parent: &parent,
			Members: []github.Member{{Login: "bob", Role: "member"}, {Login: "carol", Role: "member"}}, Repos: []github.Repo{}},
	}}
	out, err := HTML(org)
	require.NoError(t, err)
	// core: 1 direct member + 2 distinct via its subteam "web" -> badge "1+2".
	assert.Contains(t, out, "1+2")
	assert.Contains(t, out, "1 direct + 2 via subteams")
}

func TestDescendantMemberCountDedupes(t *testing.T) {
	parent := "core"
	gp := "web" // grandchild's parent
	children := map[string][]github.Team{
		"core": {{Slug: "web", Name: "Web", Parent: &parent,
			Members: []github.Member{{Login: "bob"}, {Login: "carol"}}}},
		"web": {{Slug: "web-ui", Name: "Web UI", Parent: &gp,
			Members: []github.Member{{Login: "carol"}, {Login: "dave"}}}}, // carol overlaps
	}
	// distinct across web + web-ui = bob, carol, dave = 3
	assert.Equal(t, 3, descendantMemberCount("core", children))
	assert.Equal(t, 0, descendantMemberCount("web-ui", children))
}

func TestWriteArtifacts(t *testing.T) {
	org := fixture()
	html, err := HTML(org)
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, WriteArtifacts(dir, html, org))

	idx, err := os.ReadFile(filepath.Join(dir, "index.html"))
	require.NoError(t, err)
	for _, ph := range []string{"__ORG__", "__TS__", "__TREE__", "__DATA__"} {
		assert.NotContains(t, string(idx), ph)
	}
	assert.Equal(t, html, string(idx))

	jb, err := os.ReadFile(filepath.Join(dir, "org.json"))
	require.NoError(t, err)
	var got github.Org
	require.NoError(t, json.Unmarshal(jb, &got))
	assert.Equal(t, "acme", got.Org)
	require.Len(t, got.Teams, 2)
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
