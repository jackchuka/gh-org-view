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

func TestOrgRepoAndMemberJSONRoundTrip(t *testing.T) {
	in := &Org{
		Org:         "acme",
		CollectedAt: "2026-06-18T00:00:00Z",
		Teams:       []Team{},
		Repos: []OrgRepo{{
			Name: "acme/api", Archived: true, Fork: false,
			Collaborators: []Collaborator{{Login: "carol", Permission: "admin"}},
		}},
		Members: []OrgMember{{Login: "alice", Role: "admin"}, {Login: "bob", Role: "member"}},
	}
	b, err := json.Marshal(in)
	require.NoError(t, err)

	var out Org
	require.NoError(t, json.Unmarshal(b, &out))
	require.Len(t, out.Repos, 1)
	assert.Equal(t, "acme/api", out.Repos[0].Name)
	assert.True(t, out.Repos[0].Archived)
	require.Len(t, out.Repos[0].Collaborators, 1)
	assert.Equal(t, "carol", out.Repos[0].Collaborators[0].Login)
	assert.Equal(t, "admin", out.Repos[0].Collaborators[0].Permission)
	require.Len(t, out.Members, 2)
	assert.Equal(t, "admin", out.Members[0].Role)
}
