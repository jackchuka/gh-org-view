package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectTeamsMembersRepos(t *testing.T) {
	rt := &stubRT{pages: map[string][]string{
		"/orgs/acme/teams":              {`[{"slug":"core","name":"Core","description":"d","parent":null}]`},
		"/orgs/acme/teams/core/members": {`[{"login":"alice"},{"login":"bob"}]`},
		"/orgs/acme/teams/core/repos": {
			`[{"full_name":"acme/api","archived":false,"permissions":{"admin":true,"maintain":true,"push":true}},
			  {"full_name":"acme/old","archived":true,"permissions":{"pull":true}}]`},
	}}
	// Maintainer query hits the members path with a role query; return only alice there.
	rt.pages["/orgs/acme/teams/core/members?role=maintainer"] = []string{`[{"login":"alice"}]`}

	c := testClient(t, rt)
	org, err := Collect(c, "acme", Options{Members: true, Codeowners: false})
	require.NoError(t, err)

	assert.Equal(t, "acme", org.Org)
	require.Len(t, org.Teams, 1)
	tm := org.Teams[0]
	assert.Equal(t, "Core", tm.Name)
	require.Len(t, tm.Members, 2)
	assert.Equal(t, "maintainer", roleOf(tm.Members, "alice"))
	assert.Equal(t, "member", roleOf(tm.Members, "bob"))
	require.Len(t, tm.Repos, 2)
	assert.Equal(t, "admin", tm.Repos[0].Permission) // admin wins over maintain/push
	assert.True(t, tm.Repos[1].Archived)
	assert.Equal(t, "pull", tm.Repos[1].Permission)
}

func TestCollectNoMembers(t *testing.T) {
	rt := &stubRT{pages: map[string][]string{
		"/orgs/acme/teams":            {`[{"slug":"core","name":"Core"}]`},
		"/orgs/acme/teams/core/repos": {`[]`},
	}}
	c := testClient(t, rt)
	org, err := Collect(c, "acme", Options{Members: false, Codeowners: false})
	require.NoError(t, err)
	assert.Empty(t, org.Teams[0].Members)
}

func roleOf(ms []Member, login string) string {
	for _, m := range ms {
		if m.Login == login {
			return m.Role
		}
	}
	return ""
}
