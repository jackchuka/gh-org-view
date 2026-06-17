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

// orgStub routes GraphQL POSTs by which collection they belong to, keyed by a
// substring of the query and the relevant cursor variable.
type orgStub struct{ responses map[string]string }

func (s *orgStub) RoundTrip(req *http.Request) (*http.Response, error) {
	var b struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}
	body, _ := io.ReadAll(req.Body)
	_ = json.Unmarshal(body, &b)
	cursor, _ := b.Variables["cursor"].(string)
	var key string
	switch {
	case strings.Contains(b.Query, "membersWithRole"):
		key = "members:" + cursor
	case strings.Contains(b.Query, "repositories(") && strings.Contains(b.Query, "ownerAffiliations"):
		key = "repos:" + cursor
	case strings.Contains(b.Query, "collaborators("):
		name, _ := b.Variables["name"].(string)
		key = "collab:" + name + ":" + cursor
	}
	data, ok := s.responses[key]
	if !ok {
		data = `{}`
	}
	return &http.Response{
		StatusCode: 200, Header: http.Header{},
		Body:    io.NopCloser(strings.NewReader(`{"data":` + data + `}`)),
		Request: req,
	}, nil
}

func orgTestClient(t *testing.T, s *orgStub) *Client {
	t.Helper()
	gql, err := api.NewGraphQLClient(api.ClientOptions{Host: "github.com", AuthToken: "test", Transport: s})
	require.NoError(t, err)
	return &Client{gql: gql}
}

func TestCollectReposPaginates(t *testing.T) {
	s := &orgStub{responses: map[string]string{
		"repos:": `{"organization":{"repositories":{
			"pageInfo":{"hasNextPage":true,"endCursor":"P1"},
			"nodes":[{"nameWithOwner":"acme/zeta","isArchived":false,"isFork":true}]}}}`,
		"repos:P1": `{"organization":{"repositories":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"nodes":[{"nameWithOwner":"acme/alpha","isArchived":true,"isFork":false}]}}}`,
	}}
	repos, err := collectRepos(orgTestClient(t, s), "acme")
	require.NoError(t, err)
	require.Len(t, repos, 2)
	assert.Equal(t, "acme/alpha", repos[0].Name) // sorted by name
	assert.True(t, repos[0].Archived)
	assert.Equal(t, "acme/zeta", repos[1].Name)
	assert.True(t, repos[1].Fork)
}

func TestCollectMembersPaginates(t *testing.T) {
	s := &orgStub{responses: map[string]string{
		"members:": `{"organization":{"membersWithRole":{
			"pageInfo":{"hasNextPage":true,"endCursor":"M1"},
			"edges":[{"role":"ADMIN","node":{"login":"alice"}}]}}}`,
		"members:M1": `{"organization":{"membersWithRole":{
			"pageInfo":{"hasNextPage":false,"endCursor":""},
			"edges":[{"role":"MEMBER","node":{"login":"bob"}}]}}}`,
	}}
	members, err := collectMembers(orgTestClient(t, s), "acme")
	require.NoError(t, err)
	require.Len(t, members, 2)
	assert.Equal(t, "alice", members[0].Login) // sorted by login
	assert.Equal(t, "admin", members[0].Role)
	assert.Equal(t, "bob", members[1].Login)
	assert.Equal(t, "member", members[1].Role)
}
