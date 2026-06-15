package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackchuka/gh-org-view/internal/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathUsesOrgName(t *testing.T) {
	c := New(t.TempDir())
	assert.Equal(t, "acme-org.json", filepath.Base(c.Path("acme")))
}

func TestIsFreshBoundaries(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	assert.False(t, c.IsFresh("acme"), "missing file is not fresh")

	require.NoError(t, c.Write("acme", &github.Org{Org: "acme"}))
	assert.True(t, c.IsFresh("acme"), "just-written file is fresh")

	// Backdate mtime beyond 24h.
	old := time.Now().Add(-25 * time.Hour)
	require.NoError(t, os.Chtimes(c.Path("acme"), old, old))
	assert.False(t, c.IsFresh("acme"), "file older than 24h is stale")
}

func TestWriteReadRoundTrip(t *testing.T) {
	c := New(t.TempDir())
	in := &github.Org{Org: "acme", Teams: []github.Team{{Slug: "core"}}}
	require.NoError(t, c.Write("acme", in))
	out, err := c.Read("acme")
	require.NoError(t, err)
	assert.Equal(t, "acme", out.Org)
	require.Len(t, out.Teams, 1)
	assert.Equal(t, "core", out.Teams[0].Slug)
}

func TestWriteIsAtomic(t *testing.T) {
	c := New(t.TempDir())
	require.NoError(t, c.Write("acme", &github.Org{Org: "acme"}))
	// No leftover temp file.
	matches, _ := filepath.Glob(filepath.Join(c.Dir(), "*.tmp"))
	assert.Empty(t, matches)
}
