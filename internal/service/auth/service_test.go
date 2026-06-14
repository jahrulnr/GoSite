package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/config"
	deliveryhttp "github.com/jahrulnr/gosite/internal/delivery/http"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func migrationsDir(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "..", "migrations"))
}

func setupAuthService(t *testing.T) (*auth.Service, *auth.Store, *sqlite.UserRepository) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	repo := sqlite.NewUserRepository(db)
	sessions := auth.NewStore(0)
	svc := auth.NewService(repo, sessions)

	hash, err := testutil.LaravelBcryptHash(testutil.LegacyAdminPassword)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = repo.Create(ctx, sqlite.User{
		Name:     "Admin",
		Email:    testutil.LegacyAdminEmail,
		Password: hash,
	})
	require.NoError(t, err)

	return svc, sessions, repo
}

func TestAuth_LaravelBcryptFixture(t *testing.T) {
	t.Parallel()

	hash, err := testutil.LaravelBcryptHash("secret-pass")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hash, "$2y$"))
	assert.True(t, auth.VerifyPassword("secret-pass", hash))
	assert.False(t, auth.VerifyPassword("wrong-pass", hash))
}

func TestAuth_LoginSuccess(t *testing.T) {
	t.Parallel()

	svc, sessions, _ := setupAuthService(t)
	ctx := context.Background()

	result, err := svc.Login(ctx, testutil.LegacyAdminEmail, testutil.LegacyAdminPassword, false)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	assert.Equal(t, testutil.LegacyAdminEmail, result.User.Email)

	_, ok := sessions.Get(result.Token)
	assert.True(t, ok)
}

func TestAuth_LoginBadPassword(t *testing.T) {
	t.Parallel()

	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	_, err := svc.Login(ctx, testutil.LegacyAdminEmail, "bad-password", false)
	require.Error(t, err)

	var appErr *apperror.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperror.CodeAuthInvalidCredentials, appErr.Code)
}

func TestAuth_Logout(t *testing.T) {
	t.Parallel()

	svc, sessions, _ := setupAuthService(t)
	ctx := context.Background()

	result, err := svc.Login(ctx, testutil.LegacyAdminEmail, testutil.LegacyAdminPassword, false)
	require.NoError(t, err)

	svc.Logout(result.Token)
	_, ok := sessions.Get(result.Token)
	assert.False(t, ok)
}

func TestAuth_Me(t *testing.T) {
	t.Parallel()

	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	result, err := svc.Login(ctx, testutil.LegacyAdminEmail, testutil.LegacyAdminPassword, false)
	require.NoError(t, err)

	user, err := svc.Me(ctx, result.Token)
	require.NoError(t, err)
	assert.Equal(t, testutil.LegacyAdminEmail, user.Email)
	assert.Equal(t, "Admin", user.Name)
}

func TestAuth_LoginSetsCookie(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	repo := sqlite.NewUserRepository(db)
	hash, err := testutil.LaravelBcryptHash(testutil.LegacyAdminPassword)
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), sqlite.User{
		Name:     "Admin",
		Email:    testutil.LegacyAdminEmail,
		Password: hash,
	})
	require.NoError(t, err)

	cfg := config.Config{AuthEnable: false}
	router := deliveryhttp.NewRouter(cfg, db)

	body := `{"email":"admin@demo.com","password":"123456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.NotEmpty(t, payload.Token)
	assert.Equal(t, testutil.LegacyAdminEmail, payload.User.Email)

	setCookie := rec.Header().Get("Set-Cookie")
	assert.Contains(t, setCookie, auth.SessionCookieName+"=")
}

func TestAuth_SessionUserID(t *testing.T) {
	t.Parallel()

	svc, _, _ := setupAuthService(t)
	ctx := context.Background()

	result, err := svc.Login(ctx, testutil.LegacyAdminEmail, testutil.LegacyAdminPassword, false)
	require.NoError(t, err)

	uid, ok := svc.SessionUserID(result.Token)
	assert.True(t, ok)
	assert.NotZero(t, uid)

	_, ok = svc.SessionUserID("invalid-token")
	assert.False(t, ok)
}

func TestAuth_LoginEmptyFields(t *testing.T) {
	t.Parallel()

	svc, _, _ := setupAuthService(t)
	_, err := svc.Login(context.Background(), "", "pass", false)
	require.Error(t, err)
}

func TestAuth_LoginUnknownEmail(t *testing.T) {
	t.Parallel()

	svc, _, _ := setupAuthService(t)
	_, err := svc.Login(context.Background(), "nobody@example.com", "pass", false)
	require.Error(t, err)
}

func TestAuth_MeInvalidToken(t *testing.T) {
	t.Parallel()

	svc, _, _ := setupAuthService(t)
	_, err := svc.Me(context.Background(), "bad-token")
	require.Error(t, err)
}

func TestAuth_LogoutEmptyToken(t *testing.T) {
	t.Parallel()

	svc, _, _ := setupAuthService(t)
	svc.Logout("")
}
