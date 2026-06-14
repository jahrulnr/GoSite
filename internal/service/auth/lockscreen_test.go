package auth_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockscreen_LockUnlockFlow(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrations := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations"))
	require.NoError(t, sqlite.Migrate(db, migrations))

	repo := sqlite.NewUserRepository(db)
	hash, err := testutil.LaravelBcryptHash("123456")
	require.NoError(t, err)
	_, err = repo.Create(ctx, sqlite.User{Name: "Admin", Email: "admin@demo.com", Password: hash})
	require.NoError(t, err)

	sessions := auth.NewStore(0)
	lockscreen := auth.NewLockscreen()
	svc := auth.NewService(repo, sessions, auth.WithLockscreen(lockscreen))

	result, err := svc.Login(ctx, "admin@demo.com", "123456", false)
	require.NoError(t, err)

	status, err := svc.LockscreenStatus(ctx, result.Token)
	require.NoError(t, err)
	assert.False(t, status.Locked)

	require.NoError(t, svc.LockSession(result.Token))
	status, err = svc.LockscreenStatus(ctx, result.Token)
	require.NoError(t, err)
	assert.True(t, status.Locked)
	assert.NotNil(t, status.User)

	err = svc.Unlock(ctx, result.Token, "wrong")
	require.Error(t, err)

	require.NoError(t, svc.Unlock(ctx, result.Token, "123456"))
	status, err = svc.LockscreenStatus(ctx, result.Token)
	require.NoError(t, err)
	assert.False(t, status.Locked)
}

func TestLoginMetadataFromConfig(t *testing.T) {
	meta := auth.LoginMetadataFromConfig(true, true, 300, "/www", "/storage")
	assert.True(t, meta.LockscreenEnabled)
	assert.True(t, meta.BasicAuthEnabled)
	assert.Equal(t, 300, meta.LockAfterSeconds)
	assert.Equal(t, "/www", meta.WebRoot)
	require.Len(t, meta.FileRoots, 3)
}
