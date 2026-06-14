package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepo_CreateFind(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	repo := sqlite.NewUserRepository(db)
	ctx := context.Background()

	hash, err := testutil.LaravelBcryptHash("123456")
	require.NoError(t, err)

	created, err := repo.Create(ctx, sqlite.User{
		Name:     "Admin",
		Email:    "admin@demo.com",
		Password: hash,
	})
	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, "Admin", created.Name)
	assert.Equal(t, "admin@demo.com", created.Email)

	found, err := repo.FindByEmail(ctx, "admin@demo.com")
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Email, found.Email)
	assert.Equal(t, created.Password, found.Password)

	assert.True(t, testutil.VerifyLaravelBcrypt("123456", found.Password))
}
