package database_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/database"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDB(t *testing.T) (*database.Service, *sqlite.UserRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))

	repo := sqlite.NewUserRepository(db)
	hash, err := testutil.LaravelBcryptHash(testutil.LegacyAdminPassword)
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), sqlite.User{
		Name:     "Admin",
		Email:    testutil.LegacyAdminEmail,
		Password: hash,
	})
	require.NoError(t, err)

	return database.NewService(db, dbPath), repo
}

func TestDatabase_ListTables_WhitelistOnly(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	result, err := svc.ListTables(context.Background())
	require.NoError(t, err)
	assert.Contains(t, result.Tables, "users")
	assert.Contains(t, result.Tables, "websites")
	assert.NotContains(t, result.Tables, "sqlite_sequence")
	assert.NotEmpty(t, result.Path)
}

func TestDatabase_GetTable_Users(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	data, err := svc.GetTable(context.Background(), "users", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, "users", data.Name)
	assert.Contains(t, data.Columns, "email")
	require.Len(t, data.Rows, 1)
}

func TestDatabase_GetTable_Pagination(t *testing.T) {
	t.Parallel()
	svc, repo := setupDB(t)
	ctx := context.Background()
	hash, err := testutil.LaravelBcryptHash("123456")
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = repo.Create(ctx, sqlite.User{
			Name:     "User",
			Email:    fmt.Sprintf("user%d@demo.com", i),
			Password: hash,
		})
		require.NoError(t, err)
	}

	page, err := svc.GetTable(ctx, "users", 2, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, page.Limit)
	assert.Equal(t, 1, page.Offset)
	assert.Len(t, page.Rows, 2)
}

func TestDatabase_GetTable_ForbiddenTable(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	_, err := svc.GetTable(context.Background(), "sqlite_master", 10, 0)
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeForbidden, appErr.Code)
}

func TestDatabase_GetTable_InvalidName(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	_, err := svc.GetTable(context.Background(), "users;drop", 10, 0)
	require.Error(t, err)
}

func TestDatabase_GetTable_EmptyName(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	_, err := svc.GetTable(context.Background(), " ", 10, 0)
	require.Error(t, err)
}

func TestDatabase_GetTable_DefaultLimit(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	data, err := svc.GetTable(context.Background(), "users", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 100, data.Limit)
}

func TestDatabase_GetTable_CapsLimit(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	data, err := svc.GetTable(context.Background(), "users", 9999, 0)
	require.NoError(t, err)
	assert.Equal(t, 500, data.Limit)
}

func TestDatabase_GetTable_UnknownAllowedTableEmpty(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	data, err := svc.GetTable(context.Background(), "saved_queries", 10, 0)
	require.NoError(t, err)
	assert.Empty(t, data.Rows)
	assert.NotNil(t, data.Rows)

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"rows":[]`)
	assert.NotContains(t, string(raw), `"rows":null`)
}

func TestDatabase_ListTables_Sorted(t *testing.T) {
	t.Parallel()
	svc, _ := setupDB(t)
	result, err := svc.ListTables(context.Background())
	require.NoError(t, err)
	for i := 1; i < len(result.Tables); i++ {
		assert.True(t, result.Tables[i-1] <= result.Tables[i])
	}
}
