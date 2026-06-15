package github

import (
	"fmt"
	"net/url"
	"time"
)

// Options controls what Collect gathers.
type Options struct {
	Members    bool
	Codeowners bool
}

// nowUTC is overridable in tests; production uses time.Now.
var nowUTC = func() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

type apiTeam struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Parent      *struct {
		Slug string `json:"slug"`
	} `json:"parent"`
}

type apiRepo struct {
	FullName    string `json:"full_name"`
	Archived    bool   `json:"archived"`
	Permissions struct {
		Admin    bool `json:"admin"`
		Maintain bool `json:"maintain"`
		Push     bool `json:"push"`
		Triage   bool `json:"triage"`
		Pull     bool `json:"pull"`
	} `json:"permissions"`
}

// Collect gathers the org's teams, members (with role), and repos.
func Collect(c *Client, org string, opts Options) (*Org, error) {
	apiTeams, err := paginate[apiTeam](c, fmt.Sprintf("orgs/%s/teams", url.PathEscape(org)))
	if err != nil {
		return nil, err
	}

	result := &Org{Org: org, CollectedAt: nowUTC(), Teams: make([]Team, 0, len(apiTeams))}

	for _, at := range apiTeams {
		team := Team{
			Slug:        at.Slug,
			Name:        at.Name,
			Description: at.Description,
			Members:     []Member{},
			Repos:       []Repo{},
		}
		if at.Parent != nil {
			p := at.Parent.Slug
			team.Parent = &p
		}

		if opts.Members {
			members, err := collectMembers(c, org, at.Slug)
			if err != nil {
				return nil, err
			}
			team.Members = members
		}

		repos, err := collectRepos(c, org, at.Slug)
		if err != nil {
			return nil, err
		}
		team.Repos = repos

		result.Teams = append(result.Teams, team)
	}

	if opts.Codeowners {
		if err := attachCodeowners(c, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func collectMembers(c *Client, org, slug string) ([]Member, error) {
	base := fmt.Sprintf("orgs/%s/teams/%s/members", url.PathEscape(org), url.PathEscape(slug))
	maint, err := paginate[struct {
		Login string `json:"login"`
	}](c, base+"?role=maintainer")
	if err != nil {
		return nil, err
	}
	maintainers := map[string]bool{}
	for _, m := range maint {
		maintainers[m.Login] = true
	}
	all, err := paginate[struct {
		Login string `json:"login"`
	}](c, base)
	if err != nil {
		return nil, err
	}
	members := make([]Member, 0, len(all))
	for _, m := range all {
		role := "member"
		if maintainers[m.Login] {
			role = "maintainer"
		}
		members = append(members, Member{Login: m.Login, Role: role})
	}
	return members, nil
}

func collectRepos(c *Client, org, slug string) ([]Repo, error) {
	ars, err := paginate[apiRepo](c, fmt.Sprintf("orgs/%s/teams/%s/repos", url.PathEscape(org), url.PathEscape(slug)))
	if err != nil {
		return nil, err
	}
	repos := make([]Repo, 0, len(ars))
	for _, ar := range ars {
		repos = append(repos, Repo{
			Name:       ar.FullName,
			Archived:   ar.Archived,
			Permission: permissionOf(ar),
		})
	}
	return repos, nil
}

func permissionOf(r apiRepo) string {
	switch {
	case r.Permissions.Admin:
		return "admin"
	case r.Permissions.Maintain:
		return "maintain"
	case r.Permissions.Push:
		return "push"
	case r.Permissions.Triage:
		return "triage"
	default:
		return "pull"
	}
}
