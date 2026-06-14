package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/delivery/http/middleware"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func migrationsDir(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "..", "..", "migrations"))
}

func setupSessionService(t *testing.T) (*auth.Service, *auth.Store) {
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
	_, err = repo.Create(context.Background(), sqlite.User{
		Name:     "Admin",
		Email:    testutil.LegacyAdminEmail,
		Password: hash,
	})
	require.NoError(t, err)

	return svc, sessions
}

func TestRequireSession_NoCookie(t *testing.T) {
	t.Parallel()

	svc, _ := setupSessionService(t)

	router := gin.New()
	router.GET("/api/v1/auth/me", middleware.RequireSession(svc), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var body apperror.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, apperror.CodeUnauthorized, body.Error.Code)
}

func TestRequireSession_ValidSession(t *testing.T) {
	t.Parallel()

	svc, sessions := setupSessionService(t)
	ctx := context.Background()

	result, err := svc.Login(ctx, testutil.LegacyAdminEmail, testutil.LegacyAdminPassword, false)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/api/v1/auth/me", middleware.RequireSession(svc), func(c *gin.Context) {
		assert.Equal(t, result.Token, middleware.SessionToken(c))
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{
		Name:  auth.SessionCookieName,
		Value: result.Token,
	})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	_, ok := sessions.Get(result.Token)
	assert.True(t, ok)
}
