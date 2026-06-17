package github

import "sort"

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
