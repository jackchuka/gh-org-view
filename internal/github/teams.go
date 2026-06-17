package github

import (
	"time"
)

// Options controls what Collect gathers.
type Options struct {
	Members    bool
	Codeowners bool
}

// nowUTC is overridable in tests; production uses time.Now.
var nowUTC = func() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

type gqlPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type gqlTeamsResponse struct {
	Organization struct {
		Teams struct {
			PageInfo gqlPageInfo `json:"pageInfo"`
			Nodes    []struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
				ParentTeam  *struct {
					Slug string `json:"slug"`
				} `json:"parentTeam"`
				Members struct {
					PageInfo gqlPageInfo `json:"pageInfo"`
					Edges    []struct {
						Role string `json:"role"`
						Node struct {
							Login string `json:"login"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"members"`
				Repositories struct {
					PageInfo gqlPageInfo `json:"pageInfo"`
					Edges    []struct {
						Permission string `json:"permission"`
						Node       struct {
							NameWithOwner string `json:"nameWithOwner"`
							IsArchived    bool   `json:"isArchived"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"repositories"`
			} `json:"nodes"`
		} `json:"teams"`
	} `json:"organization"`
}

// teamsQuery builds the paginated org-teams query. The members block is only
// included when membership should be collected, so Options.Members == false
// skips that work entirely.
func teamsQuery(includeMembers bool) string {
	members := ""
	if includeMembers {
		members = `
			members(first: 100, membership: IMMEDIATE) {
				pageInfo { hasNextPage endCursor }
				edges { role node { login } }
			}`
	}
	return `query($org: String!, $teamCursor: String) {
		organization(login: $org) {
			teams(first: 50, after: $teamCursor) {
				pageInfo { hasNextPage endCursor }
				nodes {
					slug name description
					parentTeam { slug }` + members + `
					repositories(first: 100) {
						pageInfo { hasNextPage endCursor }
						edges { permission node { nameWithOwner isArchived } }
					}
				}
			}
		}
	}`
}

// Collect gathers the org's teams, direct members (with role), and repos via a
// single nested, paginated GraphQL query.
func Collect(c *Client, org string, opts Options) (*Org, error) {
	result := &Org{Org: org, CollectedAt: nowUTC(), Teams: []Team{}}
	query := teamsQuery(opts.Members)
	teamCursor := ""

	for {
		var resp gqlTeamsResponse
		vars := map[string]interface{}{"org": org, "teamCursor": teamCursor}
		if err := c.gql.Do(query, vars, &resp); err != nil {
			return nil, mapAuthError(err)
		}

		for _, n := range resp.Organization.Teams.Nodes {
			team := Team{
				Slug:        n.Slug,
				Name:        n.Name,
				Description: n.Description,
				Members:     []Member{},
				Repos:       []Repo{},
			}
			if n.ParentTeam != nil {
				p := n.ParentTeam.Slug
				team.Parent = &p
			}
			for _, e := range n.Members.Edges {
				team.Members = append(team.Members, Member{Login: e.Node.Login, Role: gqlRole(e.Role)})
			}
			for _, e := range n.Repositories.Edges {
				team.Repos = append(team.Repos, Repo{
					Name:       e.Node.NameWithOwner,
					Archived:   e.Node.IsArchived,
					Permission: gqlPermission(e.Permission),
				})
			}
			if n.Members.PageInfo.HasNextPage {
				more, err := drainMembers(c, org, n.Slug, n.Members.PageInfo.EndCursor)
				if err != nil {
					return nil, err
				}
				team.Members = append(team.Members, more...)
			}
			if n.Repositories.PageInfo.HasNextPage {
				more, err := drainRepos(c, org, n.Slug, n.Repositories.PageInfo.EndCursor)
				if err != nil {
					return nil, err
				}
				team.Repos = append(team.Repos, more...)
			}
			result.Teams = append(result.Teams, team)
		}

		if !resp.Organization.Teams.PageInfo.HasNextPage {
			break
		}
		teamCursor = resp.Organization.Teams.PageInfo.EndCursor
	}

	if opts.Codeowners {
		if err := attachCodeowners(c, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

const membersDrainQuery = `query($org: String!, $slug: String!, $cursor: String!) {
	organization(login: $org) {
		team(slug: $slug) {
			members(first: 100, after: $cursor, membership: IMMEDIATE) {
				pageInfo { hasNextPage endCursor }
				edges { role node { login } }
			}
		}
	}
}`

const reposDrainQuery = `query($org: String!, $slug: String!, $cursor: String!) {
	organization(login: $org) {
		team(slug: $slug) {
			repositories(first: 100, after: $cursor) {
				pageInfo { hasNextPage endCursor }
				edges { permission node { nameWithOwner isArchived } }
			}
		}
	}
}`

func drainMembers(c *Client, org, slug, cursor string) ([]Member, error) {
	var out []Member
	for cursor != "" {
		var resp struct {
			Organization struct {
				Team struct {
					Members struct {
						PageInfo gqlPageInfo `json:"pageInfo"`
						Edges    []struct {
							Role string `json:"role"`
							Node struct {
								Login string `json:"login"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"members"`
				} `json:"team"`
			} `json:"organization"`
		}
		vars := map[string]interface{}{"org": org, "slug": slug, "cursor": cursor}
		if err := c.gql.Do(membersDrainQuery, vars, &resp); err != nil {
			return nil, mapAuthError(err)
		}
		m := resp.Organization.Team.Members
		for _, e := range m.Edges {
			out = append(out, Member{Login: e.Node.Login, Role: gqlRole(e.Role)})
		}
		if !m.PageInfo.HasNextPage || m.PageInfo.EndCursor == "" {
			break
		}
		cursor = m.PageInfo.EndCursor
	}
	return out, nil
}

func drainRepos(c *Client, org, slug, cursor string) ([]Repo, error) {
	var out []Repo
	for cursor != "" {
		var resp struct {
			Organization struct {
				Team struct {
					Repositories struct {
						PageInfo gqlPageInfo `json:"pageInfo"`
						Edges    []struct {
							Permission string `json:"permission"`
							Node       struct {
								NameWithOwner string `json:"nameWithOwner"`
								IsArchived    bool   `json:"isArchived"`
							} `json:"node"`
						} `json:"edges"`
					} `json:"repositories"`
				} `json:"team"`
			} `json:"organization"`
		}
		vars := map[string]interface{}{"org": org, "slug": slug, "cursor": cursor}
		if err := c.gql.Do(reposDrainQuery, vars, &resp); err != nil {
			return nil, mapAuthError(err)
		}
		r := resp.Organization.Team.Repositories
		for _, e := range r.Edges {
			out = append(out, Repo{
				Name:       e.Node.NameWithOwner,
				Archived:   e.Node.IsArchived,
				Permission: gqlPermission(e.Permission),
			})
		}
		if !r.PageInfo.HasNextPage || r.PageInfo.EndCursor == "" {
			break
		}
		cursor = r.PageInfo.EndCursor
	}
	return out, nil
}

// gqlPermission maps a GraphQL RepositoryPermission enum to the canonical
// permission string. The enum is the highest effective permission, so the
// boolean priority logic of the old REST path is unnecessary here.
func gqlPermission(p string) string {
	switch p {
	case "ADMIN":
		return "admin"
	case "MAINTAIN":
		return "maintain"
	case "WRITE":
		return "push"
	case "TRIAGE":
		return "triage"
	default:
		return "pull"
	}
}

// gqlRole maps a GraphQL TeamMemberRole enum to the canonical role string.
func gqlRole(r string) string {
	if r == "MAINTAINER" {
		return "maintainer"
	}
	return "member"
}
