package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCodeowners(t *testing.T) {
	text := `
# comment line
*       @acme/core
/docs   @acme/writers @alice
/api    @acme/core @other-org/team
no-owners-here
`
	got := parseCodeowners(text, "acme")
	assert.Equal(t, []string{"*", "/api"}, got["core"]) // @alice and @other-org/team ignored
	assert.Equal(t, []string{"/docs"}, got["writers"])
	assert.NotContains(t, got, "")
}

func TestAttachCodeownersOnlyOwnedRepos(t *testing.T) {
	org := &Org{
		Org: "acme",
		Teams: []Team{
			{Slug: "core", Repos: []Repo{
				{Name: "acme/api", Permission: "admin"},
				{Name: "acme/ext", Permission: "pull"}, // not owned -> not scanned
			}},
		},
	}
	rt := &stubRT{pages: map[string][]string{
		"/repos/acme/api/contents/CODEOWNERS": {"* @acme/core\n"},
	}}
	c := testClient(t, rt)
	require.NoError(t, attachCodeowners(c, org))
	assert.Equal(t, []string{"*"}, org.Teams[0].Repos[0].CodeownerPaths)
	assert.Nil(t, org.Teams[0].Repos[1].CodeownerPaths)
	// Only the owned repo's CODEOWNERS path was requested.
	assert.Equal(t, 1, rt.calls["/repos/acme/api/contents/CODEOWNERS"])
	assert.Zero(t, rt.calls["/repos/acme/ext/contents/CODEOWNERS"])
}

func TestAttachCodeownersFallbackLocation(t *testing.T) {
	org := &Org{
		Org:   "acme",
		Teams: []Team{{Slug: "core", Repos: []Repo{{Name: "acme/api", Permission: "maintain"}}}},
	}
	// Top-level CODEOWNERS is absent (404); the ".github/CODEOWNERS" fallback
	// exists. Its "/" must reach the API as a real path separator, not %2F.
	rt := &stubRT{pages: map[string][]string{
		"/repos/acme/api/contents/.github/CODEOWNERS": {"/src @acme/core\n"},
	}}
	c := testClient(t, rt)
	require.NoError(t, attachCodeowners(c, org))
	assert.Equal(t, []string{"/src"}, org.Teams[0].Repos[0].CodeownerPaths)
	assert.Equal(t, 1, rt.calls["/repos/acme/api/contents/.github/CODEOWNERS"])
}
