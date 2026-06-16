package plugin

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestConfigService(t *testing.T) (*ConfigService, *sqlite.PluginConfigRepository) {
	t.Helper()
	dbPath := t.TempDir() + "/gosite.db"
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	repo := sqlite.NewPluginConfigRepository(db)
	cipher, err := secrets.NewCipher(secrets.DerivedSource{Seed: "test"})
	require.NoError(t, err)
	return NewConfigService(repo, WithCipher(cipher)), repo
}

func TestConfigServicePutAndGetMasksSecrets(t *testing.T) {
	t.Parallel()
	svc, _ := newTestConfigService(t)
	ctx := context.Background()
	view, err := svc.Put(ctx, "acme/logger", ConfigInput{
		Version: "1.0.0",
		Values: map[string]any{
			"endpoint": "https://example.com",
			"log_level": "info",
		},
		Secrets: map[string]any{
			"api_key": "super-secret",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", view.Values["endpoint"])
	_, leaked := view.Values["api_key"]
	assert.False(t, leaked, "secret must be masked in view")
}

func TestConfigServiceRevealSecretsForRuntime(t *testing.T) {
	t.Parallel()
	svc, _ := newTestConfigService(t)
	ctx := context.Background()
	_, err := svc.Put(ctx, "acme/logger", ConfigInput{
		Version: "1.0.0",
		Values:  map[string]any{"endpoint": "https://example.com"},
		Secrets: map[string]any{"api_key": "super-secret"},
	})
	require.NoError(t, err)
	revealed, err := svc.RevealSecrets(ctx, "acme/logger", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "super-secret", revealed["api_key"])
}

func TestConfigServiceRejectsWithoutCipher(t *testing.T) {
	t.Parallel()
	dbPath := t.TempDir() + "/gosite.db"
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	repo := sqlite.NewPluginConfigRepository(db)
	svc := NewConfigService(repo)
	_, err = svc.Put(context.Background(), "acme/logger", ConfigInput{
		Version: "1.0.0",
		Values:  map[string]any{"endpoint": "x"},
	})
	require.Error(t, err)
}
