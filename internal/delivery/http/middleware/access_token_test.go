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
	"github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAccessTokenMiddleware(t *testing.T) (*plugin.IntegrationTokenService, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "..", "migrations"))))

	pluginRepo := sqlite.NewPluginRepository(db)
	tokenRepo := sqlite.NewPluginAccessTokenRepository(db)
	svc := plugin.NewIntegrationTokenService(tokenRepo, pluginRepo, nil)

	manifest := `{"id":"gosite/mcp","name":"MCP","version":"0.2.0","tier":1,"apiVersion":"gosite-plugin/1","permissions":["websites:read","system:read","cron:read","plugins:read"],"capabilities":{}}`
	_, err = pluginRepo.Create(context.Background(), sqlite.PluginVersion{
		PluginID:       "gosite/mcp",
		Version:        "0.1.0",
		Name:           "MCP",
		Tier:           1,
		APIVersion:     "gosite-plugin/1",
		ManifestJSON:   manifest,
		State:          sqlite.PluginStateEnabled,
		ArtifactPath:   "/tmp/x",
		ArtifactDigest: "abc",
	})
	require.NoError(t, err)

	result, err := svc.Create(context.Background(), "gosite/mcp", 1, plugin.CreateTokenInput{
		Label:  "test",
		Scopes: []string{"system:read"},
	}, "admin@demo.com")
	require.NoError(t, err)
	return svc, result.Plaintext
}

func TestRequireScope_ForbidsMissingScopeForAccessToken(t *testing.T) {
	t.Parallel()

	tokens, plaintext := setupAccessTokenMiddleware(t)
	authSvc := auth.NewService(nil, auth.NewStore(0))

	router := gin.New()
	router.GET(
		"/api/v1/websites",
		middleware.RequireSessionOrAccessToken(authSvc, tokens),
		middleware.RequireScope("websites:read"),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/websites", nil)
	req.Header.Set("X-Gosite-Access-Token", plaintext)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	var body apperror.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, apperror.CodeForbidden, body.Error.Code)
}

func TestRequireScope_AllowsMatchingScopeForAccessToken(t *testing.T) {
	t.Parallel()

	tokens, plaintext := setupAccessTokenMiddleware(t)
	authSvc := auth.NewService(nil, auth.NewStore(0))

	router := gin.New()
	router.GET(
		"/api/v1/system/info",
		middleware.RequireSessionOrAccessToken(authSvc, tokens),
		middleware.RequireScope("system:read"),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	req.Header.Set("X-Gosite-Access-Token", plaintext)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireScope_AllowsCronReadForAccessToken(t *testing.T) {
	t.Parallel()

	tokens, _ := setupAccessTokenMiddleware(t)
	cronResult, err := tokens.Create(context.Background(), "gosite/mcp", 1, plugin.CreateTokenInput{
		Label:  "cron",
		Scopes: []string{"cron:read"},
	}, "admin@demo.com")
	require.NoError(t, err)

	authSvc := auth.NewService(nil, auth.NewStore(0))

	router := gin.New()
	router.GET(
		"/api/v1/cronjobs",
		middleware.RequireSessionOrAccessToken(authSvc, tokens),
		middleware.RequireScope("cron:read"),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cronjobs", nil)
	req.Header.Set("X-Gosite-Access-Token", cronResult.Plaintext)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
