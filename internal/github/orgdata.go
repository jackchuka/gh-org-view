package github

import (
	"sort"
	"strings"
)

const orgReposQuery = `query($org: String!, $cursor: String) {
	organization(login: $org) {
		repositories(first: 100, after: $cursor, ownerAffiliations: [OWNER]) {
			pageInfo { hasNextPage endCursor }
			nodes { nameWithOwner isArchived isFork }
		}
	}
}`

// collectRepos returns every org-owned repository (name, archived, fork),
// sorted by name. Collaborators are filled in by a later pass.
func collectRepos(c *Client, org string) ([]OrgRepo, error) {
	var out []OrgRepo
	cursor := ""
	for {
		var resp struct {
			Organization struct {
				Repositories struct {
					PageInfo gqlPageInfo `json:"pageInfo"`
					Nodes    []struct {
						NameWithOwner string `json:"nameWithOwner"`
						IsArchived    bool   `json:"isArchived"`
						IsFork        bool   `json:"isFork"`
					} `json:"nodes"`
				} `json:"repositories"`
			} `json:"organization"`
		}
		vars := map[string]interface{}{"org": org, "cursor": cursor}
		if err := c.gql.Do(orgReposQuery, vars, &resp); err != nil {
			return nil, mapAuthError(err)
		}
		for _, n := range resp.Organization.Repositories.Nodes {
			out = append(out, OrgRepo{Name: n.NameWithOwner, Archived: n.IsArchived, Fork: n.IsFork})
		}
		pi := resp.Organization.Repositories.PageInfo
		if !pi.HasNextPage || pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

const orgMembersQuery = `query($org: String!, $cursor: String) {
	organization(login: $org) {
		membersWithRole(first: 100, after: $cursor) {
			pageInfo { hasNextPage endCursor }
			edges { role node { login } }
		}
	}
}`

// collectMembers returns every org member with role (admin|member), sorted by login.
func collectMembers(c *Client, org string) ([]OrgMember, error) {
	var out []OrgMember
	cursor := ""
	for {
		var resp struct {
			Organization struct {
				MembersWithRole struct {
					PageInfo gqlPageInfo `json:"pageInfo"`
					Edges    []struct {
						Role string `json:"role"`
						Node struct {
							Login string `json:"login"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"membersWithRole"`
			} `json:"organization"`
		}
		vars := map[string]interface{}{"org": org, "cursor": cursor}
		if err := c.gql.Do(orgMembersQuery, vars, &resp); err != nil {
			return nil, mapAuthError(err)
		}
		for _, e := range resp.Organization.MembersWithRole.Edges {
			out = append(out, OrgMember{Login: e.Node.Login, Role: orgRoleValue(e.Role)})
		}
		pi := resp.Organization.MembersWithRole.PageInfo
		if !pi.HasNextPage || pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Login < out[j].Login })
	return out, nil
}

// orgRoleValue maps a GraphQL OrganizationMemberRole enum to the canonical role.
func orgRoleValue(r string) string {
	if r == "ADMIN" {
		return "admin"
	}
	return "member"
}

const collaboratorsQuery = `query($owner: String!, $name: String!, $cursor: String) {
	repository(owner: $owner, name: $name) {
		collaborators(affiliation: DIRECT, first: 100, after: $cursor) {
			pageInfo { hasNextPage endCursor }
			edges { permission node { login } }
		}
	}
}`

// fetchCollaborators returns the DIRECT collaborators of one repo (sorted by
// login). Repos where the viewer cannot read collaborators resolve the field to
// null and yield an empty slice rather than an error, so one inaccessible repo
// never aborts collection.
func fetchCollaborators(c *Client, org, fullName string) ([]Collaborator, error) {
	owner, name, ok := strings.Cut(fullName, "/")
	if !ok {
		owner, name = org, fullName
	}
	var out []Collaborator
	var cursor interface{} // nil on first page; GitHub rejects empty string for collaborators.after
	for {
		var resp struct {
			Repository struct {
				Collaborators struct {
					PageInfo gqlPageInfo `json:"pageInfo"`
					Edges    []struct {
						Permission string `json:"permission"`
						Node       struct {
							Login string `json:"login"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"collaborators"`
			} `json:"repository"`
		}
		vars := map[string]interface{}{"owner": owner, "name": name, "cursor": cursor}
		if err := c.gql.Do(collaboratorsQuery, vars, &resp); err != nil {
			return nil, mapAuthError(err)
		}
		for _, e := range resp.Repository.Collaborators.Edges {
			out = append(out, Collaborator{Login: e.Node.Login, Permission: gqlPermission(e.Permission)})
		}
		pi := resp.Repository.Collaborators.PageInfo
		if !pi.HasNextPage || pi.EndCursor == "" {
			break
		}
		cursor = pi.EndCursor // non-empty string is valid for subsequent pages
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Login < out[j].Login })
	return out, nil
}

// attachCollaborators fills each repo's Collaborators via the worker pool.
func attachCollaborators(c *Client, org string, repos []OrgRepo) error {
	return forEachConcurrent(repos, defaultWorkers, func(i int, r OrgRepo) error {
		cols, err := fetchCollaborators(c, org, r.Name)
		if err != nil {
			return err
		}
		repos[i].Collaborators = cols
		return nil
	})
}
