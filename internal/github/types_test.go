package github

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrgUnmarshalLegacyCache(t *testing.T) {
	raw := `{
	  "org": "acme",
	  "collected_at": "2026-06-15T00:00:00Z",
	  "teams": [
	    {"slug":"core","name":"Core","description":"","parent":null,
	     "members":[{"login":"alice","role":"maintainer"}],
	     "repos":[{"name":"acme/api","archived":false,"permission":"admin","codeowner_paths":["/src"]}]}
	  ]
	}`
	var org Org
	require.NoError(t, json.Unmarshal([]byte(raw), &org))
	assert.Equal(t, "acme", org.Org)
	require.Len(t, org.Teams, 1)
	assert.Nil(t, org.Teams[0].Parent)
	assert.Equal(t, "maintainer", org.Teams[0].Members[0].Role)
	assert.Equal(t, []string{"/src"}, org.Teams[0].Repos[0].CodeownerPaths)
}

func TestOrgMarshalEmptySlicesNotNull(t *testing.T) {
	org := Org{Org: "acme", Teams: []Team{{Slug: "x", Members: []Member{}, Repos: []Repo{}}}}
	b, err := json.Marshal(org)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"members":[]`)
	assert.Contains(t, string(b), `"repos":[]`)
	assert.NotContains(t, string(b), `"codeowner_paths"`) // omitempty
}
