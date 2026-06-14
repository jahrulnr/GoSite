package settings_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/internal/service/settings"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSettings(t *testing.T) (*settings.Service, *sqlite.UserRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))

	repo := sqlite.NewUserRepository(db)
	hash, err := testutil.LaravelBcryptHash(testutil.LegacyAdminPassword)
	require.NoError(t, err)
	ctx := context.Background()
	user, err := repo.Create(ctx, sqlite.User{
		Name:     "Admin",
		Email:    testutil.LegacyAdminEmail,
		Password: hash,
	})
	require.NoError(t, err)
	require.NotZero(t, user.ID)

	return settings.NewService(repo), repo
}

func TestSettings_UpdateProfile_NameEmail(t *testing.T) {
	t.Parallel()
	svc, repo := setupSettings(t)
	ctx := context.Background()
	user, err := repo.FindByEmail(ctx, testutil.LegacyAdminEmail)
	require.NoError(t, err)

	result, err := svc.UpdateProfile(ctx, settings.ProfileInput{
		ID:    user.ID,
		Name:  "New Admin",
		Email: "new@demo.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "New Admin", result.Name)
	assert.Equal(t, "new@demo.com", result.Email)
}

func TestSettings_UpdateProfile_Password(t *testing.T) {
	t.Parallel()
	svc, repo := setupSettings(t)
	ctx := context.Background()
	user, err := repo.FindByEmail(ctx, testutil.LegacyAdminEmail)
	require.NoError(t, err)

	_, err = svc.UpdateProfile(ctx, settings.ProfileInput{
		ID:       user.ID,
		Name:     user.Name,
		Email:    user.Email,
		Password: "newpass123",
	})
	require.NoError(t, err)

	updated, err := repo.FindByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, auth.VerifyPassword("newpass123", updated.Password))
}

func TestSettings_UpdateProfile_ShortPassword(t *testing.T) {
	t.Parallel()
	svc, repo := setupSettings(t)
	ctx := context.Background()
	user, err := repo.FindByEmail(ctx, testutil.LegacyAdminEmail)
	require.NoError(t, err)

	_, err = svc.UpdateProfile(ctx, settings.ProfileInput{
		ID:       user.ID,
		Name:     user.Name,
		Email:    user.Email,
		Password: "123",
	})
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeValidation, appErr.Code)
}

func TestSettings_UpdateProfile_MissingName(t *testing.T) {
	t.Parallel()
	svc, repo := setupSettings(t)
	ctx := context.Background()
	user, err := repo.FindByEmail(ctx, testutil.LegacyAdminEmail)
	require.NoError(t, err)

	_, err = svc.UpdateProfile(ctx, settings.ProfileInput{
		ID:    user.ID,
		Name:  " ",
		Email: user.Email,
	})
	require.Error(t, err)
}

func TestSettings_UpdateProfile_InvalidEmail(t *testing.T) {
	t.Parallel()
	svc, repo := setupSettings(t)
	ctx := context.Background()
	user, err := repo.FindByEmail(ctx, testutil.LegacyAdminEmail)
	require.NoError(t, err)

	_, err = svc.UpdateProfile(ctx, settings.ProfileInput{
		ID:    user.ID,
		Name:  user.Name,
		Email: "not-an-email",
	})
	require.Error(t, err)
}

func TestSettings_UpdateProfile_UserNotFound(t *testing.T) {
	t.Parallel()
	svc, _ := setupSettings(t)
	_, err := svc.UpdateProfile(context.Background(), settings.ProfileInput{
		ID:    9999,
		Name:  "Ghost",
		Email: "ghost@demo.com",
	})
	require.Error(t, err)
	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeNotFound, appErr.Code)
}

func TestSettings_UpdateProfile_InvalidUserID(t *testing.T) {
	t.Parallel()
	svc, _ := setupSettings(t)
	_, err := svc.UpdateProfile(context.Background(), settings.ProfileInput{
		ID:    0,
		Name:  "Admin",
		Email: "admin@demo.com",
	})
	require.Error(t, err)
}

func TestSettings_ToAuthUserDTO(t *testing.T) {
	t.Parallel()
	dto := settings.ToAuthUserDTO(settings.ProfileResult{ID: 1, Name: "A", Email: "a@b.com"})
	assert.Equal(t, int64(1), dto.ID)
	assert.Equal(t, "a@b.com", dto.Email)
}

func TestSettings_UpdateProfile_EmptyPasswordKeepsHash(t *testing.T) {
	t.Parallel()
	svc, repo := setupSettings(t)
	ctx := context.Background()
	user, err := repo.FindByEmail(ctx, testutil.LegacyAdminEmail)
	require.NoError(t, err)
	before := user.Password

	_, err = svc.UpdateProfile(ctx, settings.ProfileInput{
		ID:    user.ID,
		Name:  "Renamed",
		Email: user.Email,
	})
	require.NoError(t, err)

	after, err := repo.FindByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, before, after.Password)
	assert.Equal(t, "Renamed", after.Name)
}
