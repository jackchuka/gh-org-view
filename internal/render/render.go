// Package render fills the embedded HTML UI from an Org model.
package render

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackchuka/gh-org-view/internal/github"
)

//go:embed template.html
var templateHTML string

const peopleIcon = `<svg width="14" height="14" viewBox="0 0 16 16"><path d="M2 5.5a3.5 3.5 0 1 1 5.898 2.549 5.508 5.508 0 0 1 3.034 4.084.75.75 0 1 1-1.482.235 4 4 0 0 0-7.9 0 .75.75 0 0 1-1.482-.236A5.507 5.507 0 0 1 3.102 8.05 3.493 3.493 0 0 1 2 5.5ZM11 4a3.001 3.001 0 0 1 2.22 5.018 5.01 5.01 0 0 1 2.56 3.012.749.749 0 0 1-.885.954.752.752 0 0 1-.549-.514 3.507 3.507 0 0 0-2.522-2.372.75.75 0 0 1-.574-.73v-.352a.75.75 0 0 1 .416-.672A1.5 1.5 0 0 0 11 5.5.75.75 0 0 1 11 4Zm-5.5-.5a2 2 0 1 0-.001 3.999A2 2 0 0 0 5.5 3.5Z"/></svg>`

const repoIcon = `<svg width="13" height="13" viewBox="0 0 16 16"><path d="M2 2.5A2.5 2.5 0 0 1 4.5 0h8.75a.75.75 0 0 1 .75.75v12.5a.75.75 0 0 1-.75.75h-2.5a.75.75 0 0 1 0-1.5h1.75v-2h-8a1 1 0 0 0-.714 1.7.75.75 0 1 1-1.072 1.05A2.495 2.495 0 0 1 2 11.5Zm10.5-1h-8a1 1 0 0 0-1 1v6.708A2.486 2.486 0 0 1 4.5 9h8ZM5 12.25a.25.25 0 0 1 .25-.25h3.5a.25.25 0 0 1 .25.25v3.25a.25.25 0 0 1-.4.2l-1.45-1.087a.249.249 0 0 0-.3 0L5.4 15.7a.25.25 0 0 1-.4-.2Z"/></svg>`

const extIcon = `<svg width="12" height="12" viewBox="0 0 16 16"><path d="M3.75 2h3.5a.75.75 0 0 1 0 1.5h-3.5a.25.25 0 0 0-.25.25v8.5c0 .138.112.25.25.25h8.5a.25.25 0 0 0 .25-.25v-3.5a.75.75 0 0 1 1.5 0v3.5A1.75 1.75 0 0 1 12.25 14h-8.5A1.75 1.75 0 0 1 2 12.25v-8.5C2 2.784 2.784 2 3.75 2Zm6.854-1h4.146a.25.25 0 0 1 .25.25v4.146a.25.25 0 0 1-.427.177L13.03 4.03 9.28 7.78a.751.751 0 0 1-1.06-1.06l3.75-3.75-1.543-1.543A.25.25 0 0 1 10.604 1Z"/></svg>`

// HTML renders the full self-contained explorer for org.
func HTML(org *github.Org) (string, error) {
	data, err := json.Marshal(org) // encoding/json escapes <,>,& -> safe in <script>
	if err != nil {
		return "", err
	}
	out := templateHTML
	out = strings.ReplaceAll(out, "__ORG__", htmlEscape(org.Org))
	out = strings.ReplaceAll(out, "__TS__", htmlEscape(org.CollectedAt))
	out = strings.ReplaceAll(out, "__TREE__", renderTree(org))
	out = strings.ReplaceAll(out, "__DATA__", string(data))
	return out, nil
}

// WriteArtifacts exports the rendered explorer (index.html) and the canonical
// data (org.json) into dir, creating dir if needed. html must be the output of
// HTML(org); it is passed in to avoid re-rendering.
func WriteArtifacts(dir, html string, org *github.Org) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644); err != nil {
		return err
	}
	data, err := json.MarshalIndent(org, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "org.json"), data, 0o644)
}

func renderTree(org *github.Org) string {
	bySlug := map[string]github.Team{}
	for _, t := range org.Teams {
		bySlug[t.Slug] = t
	}
	children := map[string][]github.Team{}
	var roots []github.Team
	for _, t := range org.Teams {
		if t.Parent != nil {
			if _, ok := bySlug[*t.Parent]; ok {
				children[*t.Parent] = append(children[*t.Parent], t)
				continue
			}
		}
		roots = append(roots, t)
	}
	byName := func(ts []github.Team) {
		sort.SliceStable(ts, func(i, j int) bool {
			return strings.ToLower(ts[i].Name) < strings.ToLower(ts[j].Name)
		})
	}
	byName(roots)
	for k := range children {
		byName(children[k])
	}

	var b strings.Builder
	var node func(t github.Team)
	node = func(t github.Team) {
		kids := children[t.Slug]
		chev := "&nbsp;"
		if len(kids) > 0 {
			chev = "&#9662;"
		}
		counts := `<span class="counts">` +
			metric(peopleIcon, len(t.Members), "member(s)") +
			metric(repoIcon, len(t.Repos), "repo(s)") + `</span>`
		b.WriteString(`<div class="tree-row" data-kind="team" data-id="` + htmlEscape(t.Slug) + `">`)
		b.WriteString(`<span class="chev">` + chev + `</span>`)
		b.WriteString(`<span class="ic">` + peopleIcon + `</span>`)
		b.WriteString(`<span class="name">` + htmlEscape(t.Name) + `</span>`)
		b.WriteString(`<span class="desc">` + htmlEscape(t.Description) + `</span>`)
		b.WriteString(`<span class="spacer"></span>`)
		b.WriteString(`<a class="ext" href="https://github.com/orgs/` + htmlEscape(org.Org) +
			`/teams/` + htmlEscape(t.Slug) + `" target="_blank" rel="noopener" title="Open on GitHub" data-ext>` +
			extIcon + `</a>`)
		b.WriteString(counts)
		b.WriteString(`</div>`)
		if len(kids) > 0 {
			b.WriteString(`<div class="tree-children"><div class="tree-connector"></div>`)
			for _, c := range kids {
				node(c)
			}
			b.WriteString(`</div>`)
		}
	}
	for _, r := range roots {
		node(r)
	}
	return b.String()
}

func metric(icon string, n int, label string) string {
	zero := ""
	if n == 0 {
		zero = " zero"
	}
	return fmt.Sprintf(`<span class="metric%s" title="%d %s">%s%d</span>`, zero, n, label, icon, n)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
