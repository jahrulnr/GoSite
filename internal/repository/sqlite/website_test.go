package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebsiteRepo_CreateFindList(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	repo := sqlite.NewWebsiteRepository(db)
	ctx := context.Background()

	created, err := repo.Create(ctx, sqlite.Website{
		Name:    "Demo",
		Domain:  "demo.example.com",
		Path:    "/www/demo",
		Type:    sqlite.WebsiteTypeStatic,
		Active:  true,
	})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)

	found, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "demo.example.com", found.Domain)

	byDomain, err := repo.FindByDomain(ctx, "demo.example.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, byDomain.ID)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestWebsiteRepo_UpdateDelete(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	repo := sqlite.NewWebsiteRepository(db)
	ctx := context.Background()

	site, err := repo.Create(ctx, sqlite.Website{
		Name:   "Old",
		Domain: "old.example.com",
		Path:   "/www/old",
		Type:   sqlite.WebsiteTypeProxy,
		Upstream: "http://127.0.0.1:8080",
	})
	require.NoError(t, err)

	site.Name = "New"
	site.Domain = "new.example.com"
	updated, err := repo.Update(ctx, site)
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
	assert.Equal(t, sqlite.WebsiteTypeProxy, updated.Type)

	require.NoError(t, repo.Delete(ctx, site.ID))
	_, err = repo.FindByID(ctx, site.ID)
	require.Error(t, err)
}

func TestWebsiteRepo_ExistsPathForOther(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	repo := sqlite.NewWebsiteRepository(db)
	ctx := context.Background()

	first, err := repo.Create(ctx, sqlite.Website{
		Domain: "a.example.com",
		Path:   "/www/a",
	})
	require.NoError(t, err)

	dup, err := repo.ExistsPathForOther(ctx, "/www/a", 0)
	require.NoError(t, err)
	assert.True(t, dup)

	notDup, err := repo.ExistsPathForOther(ctx, "/www/a", first.ID)
	require.NoError(t, err)
	assert.False(t, notDup)
}
