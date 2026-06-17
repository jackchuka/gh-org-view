package github

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gqlStub serves canned GraphQL responses for the /graphql POST, routing by the
// variables in the request body. Keys: "teams:<teamCursor>",
// "members:<slug>:<cursor>", "repos:<slug>:<cursor>". The stored value is the
// object placed inside {"data": ...}.
type gqlStub struct{ responses map[string]string }

func (g *gqlStub) RoundTrip(req *http.Request) (*http.Response, error) {
	var b struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}
	body, _ := io.ReadAll(req.Body)
	_ = json.Unmarshal(body, &b)
	key := gqlRouteKey(b.Query, b.Variables)
	data, ok := g.responses[key]
	if !ok {
		data = `{}`
	}
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(`{"data":` + data + `}`)),
		Request:    req,
	}, nil
}

func gqlRouteKey(query string, vars map[string]interface{}) string {
	cursor, _ := vars["cursor"].(string)
	switch {
	case strings.Contains(query, "membersWithRole"):
		return "orgmembers:" + cursor
	case strings.Contains(query, "repositories(") && strings.Contains(query, "ownerAffiliations"):
		return "orgrepos:" + cursor
	case strings.Contains(query, "collaborators("):
		name, _ := vars["name"].(string)
		return "collab:" + name + ":" + cursor
	}
	if slug, ok := vars["slug"].(string); ok {
		c, _ := vars["cursor"].(string)
		kind := "members"
		if strings.Contains(query, "repositories(") {
			kind = "repos"
		}
		return kind + ":" + slug + ":" + c
	}
	tc, _ := vars["teamCursor"].(string)
	return "teams:" + tc
}

func gqlTestClient(t *testing.T, g *gqlStub) *Client {
	t.Helper()
	gql, err := api.NewGraphQLClient(api.ClientOptions{Host: "github.com", AuthToken: "test", Transport: g})
	require.NoError(t, err)
	return &Client{gql: gql}
}

func TestCollectTeamsMembersRepos(t *testing.T) {
	g := &gqlStub{responses: map[string]string{
		"teams:": `{"organization":{"teams":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[{
				"slug":"core","name":"Core","description":"d","parentTeam":null,
				"members":{"pageInfo":{"hasNextPage":false,"endCursor":""},
					"edges":[{"role":"MAINTAINER","node":{"login":"alice"}},
					         {"role":"MEMBER","node":{"login":"bob"}}]},
				"repositories":{"pageInfo":{"hasNextPage":false,"endCursor":""},
					"edges":[{"permission":"ADMIN","node":{"nameWithOwner":"acme/api","isArchived":false}},
					         {"permission":"READ","node":{"nameWithOwner":"acme/old","isArchived":true}}]}
			}]}}}`,
	}}
	c := gqlTestClient(t, g)
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
	assert.Equal(t, "acme/api", tm.Repos[0].Name)
	assert.Equal(t, "admin", tm.Repos[0].Permission)
	assert.True(t, tm.Repos[1].Archived)
	assert.Equal(t, "pull", tm.Repos[1].Permission)
}

func TestCollectParentChildDirectMembersNotSubtracted(t *testing.T) {
	// carol is a direct member of BOTH parent (core) and child (web).
	// IMMEDIATE membership means she appears in both teams' lists.
	g := &gqlStub{responses: map[string]string{
		"teams:": `{"organization":{"teams":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[
				{"slug":"core","name":"Core","description":"","parentTeam":null,
				 "members":{"pageInfo":{"hasNextPage":false,"endCursor":""},
					"edges":[{"role":"MEMBER","node":{"login":"carol"}}]},
				 "repositories":{"pageInfo":{"hasNextPage":false,"endCursor":""},"edges":[]}},
				{"slug":"web","name":"Web","description":"","parentTeam":{"slug":"core"},
				 "members":{"pageInfo":{"hasNextPage":false,"endCursor":""},
					"edges":[{"role":"MEMBER","node":{"login":"carol"}}]},
				 "repositories":{"pageInfo":{"hasNextPage":false,"endCursor":""},"edges":[]}}
			]}}}`,
	}}
	c := gqlTestClient(t, g)
	org, err := Collect(c, "acme", Options{Members: true})
	require.NoError(t, err)
	require.Len(t, org.Teams, 2)
	assert.Equal(t, "member", roleOf(org.Teams[0].Members, "carol"))
	assert.Equal(t, "member", roleOf(org.Teams[1].Members, "carol"))
	require.NotNil(t, org.Teams[1].Parent)
	assert.Equal(t, "core", *org.Teams[1].Parent)
}

func TestCollectNoMembers(t *testing.T) {
	g := &gqlStub{responses: map[string]string{
		"teams:": `{"organization":{"teams":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[{"slug":"core","name":"Core","description":"","parentTeam":null,
				"repositories":{"pageInfo":{"hasNextPage":false,"endCursor":""},"edges":[]}}]}}}`,
	}}
	c := gqlTestClient(t, g)
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

func TestCollectDrainsMemberAndRepoPages(t *testing.T) {
	g := &gqlStub{responses: map[string]string{
		// First teams page: team "big" has more members AND more repos.
		"teams:": `{"organization":{"teams":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[{"slug":"big","name":"Big","description":"","parentTeam":null,
				"members":{"pageInfo":{"hasNextPage":true,"endCursor":"M1"},
					"edges":[{"role":"MEMBER","node":{"login":"a"}}]},
				"repositories":{"pageInfo":{"hasNextPage":true,"endCursor":"R1"},
					"edges":[{"permission":"WRITE","node":{"nameWithOwner":"acme/r1","isArchived":false}}]}
			}]}}}`,
		// Member drain page 2 (no further pages).
		"members:big:M1": `{"organization":{"team":{"members":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"edges":[{"role":"MAINTAINER","node":{"login":"b"}}]}}}}`,
		// Repo drain page 2 (no further pages).
		"repos:big:R1": `{"organization":{"team":{"repositories":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"edges":[{"permission":"READ","node":{"nameWithOwner":"acme/r2","isArchived":true}}]}}}}`,
	}}
	c := gqlTestClient(t, g)
	org, err := Collect(c, "acme", Options{Members: true})
	require.NoError(t, err)
	require.Len(t, org.Teams, 1)
	tm := org.Teams[0]
	require.Len(t, tm.Members, 2)
	assert.Equal(t, "member", roleOf(tm.Members, "a"))
	assert.Equal(t, "maintainer", roleOf(tm.Members, "b"))
	require.Len(t, tm.Repos, 2)
	assert.Equal(t, "push", tm.Repos[0].Permission)
	assert.Equal(t, "acme/r2", tm.Repos[1].Name)
	assert.True(t, tm.Repos[1].Archived)
}

func TestGqlPermission(t *testing.T) {
	cases := map[string]string{
		"ADMIN":    "admin",
		"MAINTAIN": "maintain",
		"WRITE":    "push",
		"TRIAGE":   "triage",
		"READ":     "pull",
		"":         "pull", // unknown/empty falls back to least privilege
	}
	for in, want := range cases {
		assert.Equal(t, want, gqlPermission(in))
	}
}

func TestGqlRole(t *testing.T) {
	assert.Equal(t, "maintainer", gqlRole("MAINTAINER"))
	assert.Equal(t, "member", gqlRole("MEMBER"))
	assert.Equal(t, "member", gqlRole(""))
}

func TestCollectPopulatesOrgReposAndMembers(t *testing.T) {
	g := &gqlStub{responses: map[string]string{
		"teams:": `{"organization":{"teams":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[{"slug":"core","name":"Core","description":"","parentTeam":null,
				"members":{"pageInfo":{"hasNextPage":false,"endCursor":""},"edges":[]},
				"repositories":{"pageInfo":{"hasNextPage":false,"endCursor":""},"edges":[]}}]}}}`,
		"orgrepos:": `{"organization":{"repositories":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[{"nameWithOwner":"acme/api","isArchived":false,"isFork":false},
			         {"nameWithOwner":"acme/orphan","isArchived":false,"isFork":false}]}}}`,
		"orgmembers:": `{"organization":{"membersWithRole":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"edges":[{"role":"ADMIN","node":{"login":"alice"}}]}}}`,
		"collab:api:": `{"repository":{"collaborators":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"edges":[{"permission":"WRITE","node":{"login":"dave"}}]}}}`,
		// acme/orphan: no collab stub -> empty.
	}}
	c := gqlTestClient(t, g)
	org, err := Collect(c, "acme", Options{Members: true, Codeowners: false})
	require.NoError(t, err)
	require.Len(t, org.Repos, 2)
	assert.Equal(t, "acme/api", org.Repos[0].Name)
	require.Len(t, org.Repos[0].Collaborators, 1)
	assert.Equal(t, "dave", org.Repos[0].Collaborators[0].Login)
	assert.Equal(t, "push", org.Repos[0].Collaborators[0].Permission)
	assert.Empty(t, org.Repos[1].Collaborators)
	require.Len(t, org.Members, 1)
	assert.Equal(t, "alice", org.Members[0].Login)
	assert.Equal(t, "admin", org.Members[0].Role)
}
