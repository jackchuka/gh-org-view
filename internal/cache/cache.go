// Package cache stores the canonical org JSON on disk and treats it as a 24h
// cache that is also hand-editable between runs.
package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackchuka/gh-org-view/internal/github"
)

const maxAge = 24 * time.Hour

// Cache resolves and reads/writes <org>-org.json under a directory.
type Cache struct{ dir string }

// New returns a Cache rooted at dir.
func New(dir string) *Cache { return &Cache{dir: dir} }

// DefaultDir is ${TMPDIR:-/tmp}/gh-org-view.
func DefaultDir() string {
	base := os.TempDir()
	return filepath.Join(base, "gh-org-view")
}

func (c *Cache) Dir() string { return c.dir }

func (c *Cache) Path(org string) string {
	return filepath.Join(c.dir, org+"-org.json")
}

// IsFresh reports whether the cache file exists and is at most 24h old.
func (c *Cache) IsFresh(org string) bool {
	info, err := os.Stat(c.Path(org))
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) <= maxAge
}

// Read loads the cached org.
func (c *Cache) Read(org string) (*github.Org, error) {
	data, err := os.ReadFile(c.Path(org))
	if err != nil {
		return nil, err
	}
	var o github.Org
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("parse cache %s: %w", c.Path(org), err)
	}
	return &o, nil
}

// Write atomically persists the org (temp file + rename).
func (c *Cache) Write(org string, o *github.Org) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	path := c.Path(org)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
