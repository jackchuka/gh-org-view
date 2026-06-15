package render

import (
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
	assert.NotContains(t, out, "</script>\n<")
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
