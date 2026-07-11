package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/delivery/http/handler"
	"github.com/jahrulnr/gosite/internal/delivery/http/middleware"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/jahrulnr/gosite/internal/service/plugin/catalog"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupPluginListRouter(t *testing.T) (*gin.Engine, *plugin.IntegrationTokenService, string, string) {
	t.Helper()

	dbPath := t.TempDir() + "/gosite.db"
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	ctx := context.Background()

	userRepo := sqlite.NewUserRepository(db)
	hash, err := testutil.LaravelBcryptHash(testutil.LegacyAdminPassword)
	require.NoError(t, err)
	_, err = userRepo.Create(ctx, sqlite.User{
		Name:     "Admin",
		Email:    testutil.LegacyAdminEmail,
		Password: hash,
	})
	require.NoError(t, err)

	sessions := auth.NewStore(0)
	authSvc := auth.NewService(userRepo, sessions)

	pluginRepo := sqlite.NewPluginRepository(db)
	tokenRepo := sqlite.NewPluginAccessTokenRepository(db)
	tokens := plugin.NewIntegrationTokenService(tokenRepo, pluginRepo, nil)

	manifest := `{"id":"gosite/mcp","name":"MCP","version":"0.2.0","tier":1,"apiVersion":"gosite-plugin/1","permissions":["system:read","plugins:read"],"capabilities":{}}`
	_, err = pluginRepo.Create(ctx, sqlite.PluginVersion{
		PluginID:       "gosite/mcp",
		Version:        "0.2.0",
		Name:           "MCP",
		Tier:           1,
		APIVersion:     "gosite-plugin/1",
		ManifestJSON:   manifest,
		State:          sqlite.PluginStateEnabled,
		ArtifactPath:   "/tmp/x",
		ArtifactDigest: "abc",
	})
	require.NoError(t, err)

	withPlugins, err := tokens.Create(ctx, "gosite/mcp", 1, plugin.CreateTokenInput{
		Label:  "plugins",
		Scopes: []string{"plugins:read"},
	}, testutil.LegacyAdminEmail)
	require.NoError(t, err)

	withoutPlugins, err := tokens.Create(ctx, "gosite/mcp", 1, plugin.CreateTokenInput{
		Label:  "system",
		Scopes: []string{"system:read"},
	}, testutil.LegacyAdminEmail)
	require.NoError(t, err)

	pluginSvc := plugin.NewService(pluginRepo, t.TempDir(), nil, nil)
	pluginHandler := handler.NewPluginHandler(pluginSvc, nil, remote.Config{}, catalog.NewService(""))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(
		"/api/v1/plugins",
		middleware.RequireSessionOrAccessToken(authSvc, tokens),
		middleware.RequireScope("plugins:read"),
		gin.WrapF(pluginHandler.List),
	)

	return router, tokens, withPlugins.Plaintext, withoutPlugins.Plaintext
}

func TestPluginList_WithPluginsReadScope_Returns200(t *testing.T) {
	t.Parallel()

	router, _, token, _ := setupPluginListRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil)
	req.Header.Set("X-Gosite-Access-Token", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "plugins")
}

func TestPluginList_WithoutPluginsReadScope_Returns403(t *testing.T) {
	t.Parallel()

	router, _, _, token := setupPluginListRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil)
	req.Header.Set("X-Gosite-Access-Token", token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPluginList_WithSession_Returns200(t *testing.T) {
	t.Parallel()

	dbPath := t.TempDir() + "/gosite.db"
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	ctx := context.Background()
	userRepo := sqlite.NewUserRepository(db)
	hash, err := testutil.LaravelBcryptHash(testutil.LegacyAdminPassword)
	require.NoError(t, err)
	user, err := userRepo.Create(ctx, sqlite.User{
		Name:     "Admin",
		Email:    testutil.LegacyAdminEmail,
		Password: hash,
	})
	require.NoError(t, err)

	sessions := auth.NewStore(0)
	authSvc := auth.NewService(userRepo, sessions)
	session, err := sessions.CreateFor(user.ID, false)
	require.NoError(t, err)

	pluginRepo := sqlite.NewPluginRepository(db)
	pluginSvc := plugin.NewService(pluginRepo, t.TempDir(), nil, nil)
	pluginHandler := handler.NewPluginHandler(pluginSvc, nil, remote.Config{}, catalog.NewService(""))

	tokens := plugin.NewIntegrationTokenService(sqlite.NewPluginAccessTokenRepository(db), pluginRepo, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(
		"/api/v1/plugins",
		middleware.RequireSessionOrAccessToken(authSvc, tokens),
		middleware.RequireScope("plugins:read"),
		gin.WrapF(pluginHandler.List),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: session.ID})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
