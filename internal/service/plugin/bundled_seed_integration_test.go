package plugin_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/stretchr/testify/require"
)

func TestSeedBundled_OfficialMCPZip(t *testing.T) {
	zipPath := filepath.Clean(filepath.Join("..", "..", "..", "dist", "bundled-plugins", "gosite-mcp.zip"))
	if _, err := os.Stat(zipPath); err != nil {
		t.Skip("run make bundled-plugins first")
	}
	dir := filepath.Dir(zipPath)
	svc, repo := setupPluginServiceWithOptions(t, plugin.NoopRuntimeManager{}, plugin.NewMemoryHookDispatcher(4),
		plugin.WithBundled(dir, true, false, "local"),
		plugin.WithHostVersion("9.9.9"),
	)
	require.NoError(t, svc.SeedBundled(context.Background()))
	rows, err := repo.List(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	found := false
	for _, row := range rows {
		if row.PluginID == "gosite/mcp" {
			found = true
			require.Equal(t, sqlite.PluginStateInstalled, row.State)
			require.Equal(t, "bundled", row.SourceType)
		}
	}
	require.True(t, found, "gosite/mcp should be seeded")
}
