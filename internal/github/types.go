// Package github collects an organization's team/member/repo/CODEOWNERS topology
// from the GitHub GraphQL API (CODEOWNERS file contents via REST) into a
// canonical, JSON-serializable model.
package github

// Org is the canonical, hand-editable data model. Its JSON shape matches the
// legacy /gh-org-chart cache so existing <org>-org.json files remain readable.
type Org struct {
	Org         string      `json:"org"`
	CollectedAt string      `json:"collected_at"`
	Teams       []Team      `json:"teams"`
	Repos       []OrgRepo   `json:"repos"`
	Members     []OrgMember `json:"members"`
}

type Team struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Parent      *string  `json:"parent"` // parent team slug, or null
	Members     []Member `json:"members"`
	Repos       []Repo   `json:"repos"`
}

type Member struct {
	Login string `json:"login"`
	Role  string `json:"role"` // "maintainer" | "member"
}

type Repo struct {
	Name           string   `json:"name"` // full name, "owner/repo"
	Archived       bool     `json:"archived"`
	Permission     string   `json:"permission"` // admin|maintain|push|triage|pull
	CodeownerPaths []string `json:"codeowner_paths,omitempty"`
}

// OrgRepo is the org-wide repo record. Team grants for a repo continue to live
// under Teams[].Repos; Collaborators holds only DIRECT grants (a user added
// straight to the repo, not via a team or org membership).
type OrgRepo struct {
	Name          string         `json:"name"` // full name, "owner/repo"
	Archived      bool           `json:"archived"`
	Fork          bool           `json:"fork"`
	Collaborators []Collaborator `json:"collaborators,omitempty"`
}

type Collaborator struct {
	Login      string `json:"login"`
	Permission string `json:"permission"` // admin|maintain|push|triage|pull
}

type OrgMember struct {
	Login string `json:"login"`
	Role  string `json:"role"` // "admin" (owner) | "member"
}
