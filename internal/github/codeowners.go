package github

import (
	"bufio"
	"fmt"
	"strings"
)

var codeownersLocations = []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"}

// attachCodeowners scans CODEOWNERS for owned repos (admin/maintain) and fills
// each repo's CodeownerPaths with the patterns owned by that team.
func attachCodeowners(c *Client, org *Org) error {
	// Set of owned repo full names.
	owned := map[string]bool{}
	for _, t := range org.Teams {
		for _, r := range t.Repos {
			if r.Permission == "admin" || r.Permission == "maintain" {
				owned[r.Name] = true
			}
		}
	}

	// repo full name -> (team slug -> patterns)
	attr := map[string]map[string][]string{}
	for full := range owned {
		text, ok, err := fetchCodeowners(c, full)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		attr[full] = parseCodeowners(text, org.Org)
	}

	// Write back onto each team's repos.
	for ti := range org.Teams {
		t := &org.Teams[ti]
		for ri := range t.Repos {
			r := &t.Repos[ri]
			if paths, ok := attr[r.Name][t.Slug]; ok {
				r.CodeownerPaths = paths
			}
		}
	}
	return nil
}

func fetchCodeowners(c *Client, fullName string) (string, bool, error) {
	for _, loc := range codeownersLocations {
		// loc is a hardcoded constant; passing it unescaped keeps the "/" in
		// ".github/CODEOWNERS" as a real path separator (escaping it to %2F
		// makes GitHub's Contents API 404).
		path := fmt.Sprintf("repos/%s/contents/%s", fullName, loc)
		b, ok, err := c.getRaw(path)
		if err != nil {
			return "", false, err
		}
		if ok {
			return string(b), true, nil
		}
	}
	return "", false, nil
}

// parseCodeowners returns {team-slug: [pattern,...]} for @<org>/<slug> owners
// only. Individual users (@alice) and external orgs (@other/team) are ignored.
func parseCodeowners(text, org string) map[string][]string {
	prefix := "@" + org + "/"
	out := map[string][]string{}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pattern := fields[0]
		for _, owner := range fields[1:] {
			slug, ok := strings.CutPrefix(owner, prefix)
			if !ok || slug == "" {
				continue
			}
			out[slug] = append(out[slug], pattern)
		}
	}
	return out
}
