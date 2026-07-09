package plugin_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupIntegrationTokenService(t *testing.T) (*plugin.IntegrationTokenService, *sqlite.PluginRepository, *sqlite.PluginAccessTokenRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	pluginRepo := sqlite.NewPluginRepository(db)
	tokenRepo := sqlite.NewPluginAccessTokenRepository(db)
	return plugin.NewIntegrationTokenService(tokenRepo, pluginRepo, nil), pluginRepo, tokenRepo
}

func TestIntegrationToken_CreateAndIntrospect(t *testing.T) {
	svc, pluginRepo, _ := setupIntegrationTokenService(t)
	ctx := context.Background()

	manifest := `{"id":"gosite/mcp","name":"MCP","version":"0.1.0","tier":1,"apiVersion":"gosite-plugin/1","permissions":["websites:read","system:read"],"capabilities":{}}`
	_, err := pluginRepo.Create(ctx, sqlite.PluginVersion{
		PluginID:     "gosite/mcp",
		Version:      "0.1.0",
		Name:         "MCP",
		Tier:         1,
		APIVersion:   "gosite-plugin/1",
		ManifestJSON: manifest,
		State:        sqlite.PluginStateEnabled,
		ArtifactPath: "/tmp/x",
		ArtifactDigest: "abc",
	})
	require.NoError(t, err)

	result, err := svc.Create(ctx, "gosite/mcp", 1, plugin.CreateTokenInput{
		Label:  "cursor",
		Scopes: []string{"websites:read"},
	}, "admin@demo.com")
	require.NoError(t, err)
	assert.True(t, len(result.Plaintext) > 10)

	row, scopes, err := svc.Introspect(ctx, result.Plaintext)
	require.NoError(t, err)
	assert.Equal(t, "gosite/mcp", row.PluginID)
	assert.Equal(t, []string{"websites:read"}, scopes)
}

func TestIntegrationToken_ReconcileAfterSwitch(t *testing.T) {
	svc, pluginRepo, tokenRepo := setupIntegrationTokenService(t)
	ctx := context.Background()

	oldManifest := `{"id":"gosite/mcp","permissions":["websites:read","docker:manage"]}`
	newManifest := plugin.Manifest{ID: "gosite/mcp", Permissions: []string{"websites:read"}}

	_, err := pluginRepo.Create(ctx, sqlite.PluginVersion{
		PluginID: "gosite/mcp", Version: "0.1.0", Name: "MCP", Tier: 1, APIVersion: "gosite-plugin/1",
		ManifestJSON: oldManifest, State: sqlite.PluginStateEnabled, ArtifactPath: "/tmp", ArtifactDigest: "x",
	})
	require.NoError(t, err)

	created, err := svc.Create(ctx, "gosite/mcp", 1, plugin.CreateTokenInput{
		Label: "bot", Scopes: []string{"websites:read", "docker:manage"},
	}, "admin@demo.com")
	require.NoError(t, err)

	require.NoError(t, svc.ReconcileAfterSwitch(ctx, "gosite/mcp", newManifest, "admin@demo.com"))
	updated, err := tokenRepo.FindByID(ctx, created.Token.ID)
	require.NoError(t, err)
	scopes, err := plugin.DecodeTokenScopes(updated.ScopesJSON)
	require.NoError(t, err)
	assert.Equal(t, []string{"websites:read"}, scopes)
}

func TestIntegrationToken_RevokeBlocksAuth(t *testing.T) {
	svc, pluginRepo, _ := setupIntegrationTokenService(t)
	ctx := context.Background()
	manifest := `{"id":"gosite/mcp","permissions":["system:read"]}`
	_, err := pluginRepo.Create(ctx, sqlite.PluginVersion{
		PluginID: "gosite/mcp", Version: "0.1.0", Name: "MCP", Tier: 1, APIVersion: "gosite-plugin/1",
		ManifestJSON: manifest, State: sqlite.PluginStateEnabled, ArtifactPath: "/tmp", ArtifactDigest: "x",
	})
	require.NoError(t, err)
	created, err := svc.Create(ctx, "gosite/mcp", 1, plugin.CreateTokenInput{Label: "x", Scopes: []string{"system:read"}}, "admin@demo.com")
	require.NoError(t, err)
	_, err = svc.Revoke(ctx, "gosite/mcp", created.Token.ID, "admin@demo.com")
	require.NoError(t, err)
	_, _, err = svc.Introspect(ctx, created.Plaintext)
	require.Error(t, err)
}

func TestIntegrationToken_ExpiresAt(t *testing.T) {
	svc, pluginRepo, _ := setupIntegrationTokenService(t)
	ctx := context.Background()
	manifest := `{"id":"gosite/mcp","permissions":["system:read"]}`
	_, err := pluginRepo.Create(ctx, sqlite.PluginVersion{
		PluginID: "gosite/mcp", Version: "0.1.0", Name: "MCP", Tier: 1, APIVersion: "gosite-plugin/1",
		ManifestJSON: manifest, State: sqlite.PluginStateEnabled, ArtifactPath: "/tmp", ArtifactDigest: "x",
	})
	require.NoError(t, err)
	past := time.Now().UTC().Add(-time.Hour)
	created, err := svc.Create(ctx, "gosite/mcp", 1, plugin.CreateTokenInput{
		Label: "expired", Scopes: []string{"system:read"}, ExpiresAt: &past,
	}, "admin@demo.com")
	require.NoError(t, err)
	_, _, err = svc.Introspect(ctx, created.Plaintext)
	require.Error(t, err)
}
