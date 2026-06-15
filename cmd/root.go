// Package cmd implements the gh-org-view CLI.
package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackchuka/gh-org-view/internal/cache"
	"github.com/jackchuka/gh-org-view/internal/github"
	"github.com/jackchuka/gh-org-view/internal/render"
	"github.com/spf13/cobra"
)

var (
	flagRefresh      bool
	flagNoCodeowners bool
	flagNoMembers    bool
	flagJSON         bool
	flagNoOpen       bool
)

var rootCmd = &cobra.Command{
	Use:   "org-view <org>",
	Short: "Explore a GitHub org's teams, members, repos, and CODEOWNERS",
	Long: `gh-org-view collects a GitHub organization's teams (with members and
roles), owned repositories, and CODEOWNERS path attributions into a canonical
JSON file, then renders a self-contained interactive HTML explorer.

The JSON in ${TMPDIR:-/tmp}/gh-org-view/<org>-org.json is a 24h cache and is
hand-editable; edits survive re-runs unless --refresh is passed.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runRoot,
}

func init() {
	f := rootCmd.Flags()
	f.BoolVar(&flagRefresh, "refresh", false, "Force re-collection, ignoring the cache")
	f.BoolVar(&flagNoCodeowners, "no-codeowners", false, "Skip the CODEOWNERS scan (faster)")
	f.BoolVar(&flagNoMembers, "no-members", false, "Skip collecting team members")
	f.BoolVar(&flagJSON, "json", false, "Print the canonical JSON to stdout; skip HTML render and open")
	f.BoolVar(&flagNoOpen, "no-open", false, "Skip opening the HTML in a browser")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	org := args[0]
	c := cache.New(cache.DefaultDir())

	var data *github.Org
	var fromCache bool
	if c.IsFresh(org) && !flagRefresh {
		d, err := c.Read(org)
		if err != nil {
			return err
		}
		data, fromCache = d, true
	} else {
		client, err := github.NewClient()
		if err != nil {
			return err
		}
		s := newSpinner(cmd.ErrOrStderr(), fmt.Sprintf("Collecting %s…", org))
		s.Start()
		d, err := github.Collect(client, org, github.Options{
			Members:    !flagNoMembers,
			Codeowners: !flagNoCodeowners,
		})
		s.Stop()
		if err != nil {
			return err
		}
		if err := c.Write(org, d); err != nil {
			return err
		}
		data = d
	}

	if flagJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	html, err := render.HTML(data)
	if err != nil {
		return err
	}
	htmlPath := strings.TrimSuffix(c.Path(org), "-org.json") + "-org.html"
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		return err
	}

	if !flagNoOpen {
		if err := openBrowser(htmlPath); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not open browser: %v\n", err)
		}
	}

	printSummary(cmd.ErrOrStderr(), data, htmlPath, fromCache)
	return nil
}

func printSummary(w io.Writer, org *github.Org, htmlPath string, fromCache bool) {
	teams := len(org.Teams)
	ownedSet := map[string]bool{}
	members, attributions := 0, 0
	for _, t := range org.Teams {
		members += len(t.Members)
		for _, r := range t.Repos {
			if r.Permission == "admin" || r.Permission == "maintain" {
				ownedSet[r.Name] = true
			}
			attributions += len(r.CodeownerPaths)
		}
	}
	src := "freshly collected"
	if fromCache {
		src = "reused from cache"
	}
	fmt.Fprintf(w, "\norg-view for %s (%s)\n", org.Org, src)
	fmt.Fprintf(w, "  teams:                  %d\n", teams)
	fmt.Fprintf(w, "  owned repos:            %d\n", len(ownedSet))
	fmt.Fprintf(w, "  member entries:         %d\n", members)
	fmt.Fprintf(w, "  codeowner attributions: %d\n", attributions)
	fmt.Fprintf(w, "  html:                   %s\n", htmlPath)
}
