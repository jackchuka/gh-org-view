# gh-org-view

[![Test](https://github.com/jackchuka/gh-org-view/actions/workflows/test.yml/badge.svg)](https://github.com/jackchuka/gh-org-view/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/jackchuka/gh-org-view?sort=semver)](https://github.com/jackchuka/gh-org-view/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A GitHub CLI extension that renders an interactive, offline HTML explorer of a
GitHub organization's teams, members (with roles), owned repositories, and
CODEOWNERS path attributions.

> Note: this is a permission/ownership *view*, not a management org chart.
> GitHub teams reflect access grouping, not reporting lines.

## Installation

```bash
gh extension install jackchuka/gh-org-view
```

Requires [GitHub CLI](https://cli.github.com/) authenticated with the `read:org`
scope (`gh auth refresh -s read:org`).

## Usage

```bash
gh org-view <org>                 # collect (or reuse 24h cache) and open the explorer
gh org-view <org> --refresh       # force re-collection
gh org-view <org> --no-codeowners # skip the CODEOWNERS scan (faster on big orgs)
gh org-view <org> --no-members    # skip collecting members
gh org-view <org> --json          # print canonical JSON to stdout (for jq/scripting)
gh org-view <org> --no-open       # render but do not open a browser
gh org-view <org> --out-dir ./site # write index.html + org.json into ./site (for CI/static hosting)
```

Artifacts are written to `${TMPDIR:-/tmp}/gh-org-view/`:
`<org>-org.json` (canonical, hand-editable, 24h cache) and `<org>-org.html`
(self-contained explorer).

## License

MIT
