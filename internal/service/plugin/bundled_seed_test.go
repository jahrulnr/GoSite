package plugin_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeBundledDir(t *testing.T, manifest string, artifactName string, artifact []byte) string {
	t.Helper()
	dir := t.TempDir()
	index := `{
		"apiVersion":"gosite-plugin-bundled/1",
		"plugins":[{"plugin_id":"gosite/demo","artifact":"` + artifactName + `","permissions_pre_ack":true,"restorable":true}]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundled-plugins.json"), []byte(index), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, artifactName), artifact, 0o644))
	return dir
}

func TestSeedBundled_InstallsTier0Plugin(t *testing.T) {
	artifact := zipArtifact(t, manifestJSON("gosite/demo", "1.0.0", 0), nil)
	dir := writeBundledDir(t, manifestJSON("gosite/demo", "1.0.0", 0), "demo.zip", artifact)

	svc, repo := setupPluginServiceWithOptions(t, plugin.NoopRuntimeManager{}, plugin.NewMemoryHookDispatcher(4),
		plugin.WithBundled(dir, true, false, "local"),
		plugin.WithAllowUnsigned(false),
	)

	ctx := context.Background()
	require.NoError(t, svc.SeedBundled(ctx))
	require.NoError(t, svc.SeedBundled(ctx))

	rows, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "gosite/demo", rows[0].PluginID)
	assert.Equal(t, sqlite.PluginStateInstalled, rows[0].State)
	assert.Equal(t, "bundled", rows[0].SourceType)
	assert.NotNil(t, rows[0].PermissionsAckAt)
}

func TestSeedBundled_SkipsWhenDisabled(t *testing.T) {
	artifact := zipArtifact(t, manifestJSON("gosite/demo", "1.0.0", 0), nil)
	dir := writeBundledDir(t, "", "demo.zip", artifact)
	svc, repo := setupPluginServiceWithOptions(t, plugin.NoopRuntimeManager{}, plugin.NewMemoryHookDispatcher(4),
		plugin.WithBundled(dir, false, false, "local"),
	)
	require.NoError(t, svc.SeedBundled(context.Background()))
	rows, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestRestoreBundled_ReinstallsUninstalled(t *testing.T) {
	artifact := zipArtifact(t, manifestJSON("gosite/demo", "1.0.0", 0), nil)
	dir := writeBundledDir(t, "", "demo.zip", artifact)
	svc, _ := setupPluginServiceWithOptions(t, plugin.NoopRuntimeManager{}, plugin.NewMemoryHookDispatcher(4),
		plugin.WithBundled(dir, true, false, "local"),
	)
	ctx := context.Background()
	require.NoError(t, svc.SeedBundled(ctx))
	_, err := svc.Uninstall(ctx, "gosite/demo", "1.0.0")
	require.NoError(t, err)
	restored, err := svc.RestoreBundled(ctx, "gosite/demo")
	require.NoError(t, err)
	assert.Equal(t, sqlite.PluginStateInstalled, restored.State)
	assert.Equal(t, "bundled", restored.SourceType)
}

func TestReconcile_AutoRestoresBundledPlugin(t *testing.T) {
	artifact := zipArtifact(t, manifestJSON("gosite/demo", "1.0.0", 0), nil)
	dir := writeBundledDir(t, "", "demo.zip", artifact)
	svc, repo := setupPluginServiceWithOptions(t, plugin.NoopRuntimeManager{}, plugin.NewMemoryHookDispatcher(4),
		plugin.WithBundled(dir, true, false, "local"),
	)
	ctx := context.Background()
	require.NoError(t, svc.SeedBundled(ctx))
	_, err := svc.Uninstall(ctx, "gosite/demo", "1.0.0")
	require.NoError(t, err)

	require.NoError(t, svc.Reconcile(ctx))

	rows, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, sqlite.PluginStateInstalled, rows[0].State)
	assert.Equal(t, "bundled", rows[0].SourceType)
}

func TestSeedBundled_PreAckForGoSiteVendor(t *testing.T) {
	artifact := zipArtifact(t, manifestJSON("gosite/demo", "1.0.0", 0), nil)
	dir := t.TempDir()
	index := `{
		"apiVersion":"gosite-plugin-bundled/1",
		"plugins":[{"plugin_id":"gosite/demo","artifact":"demo.zip","permissions_pre_ack":false,"restorable":true}]
	}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundled-plugins.json"), []byte(index), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "demo.zip"), artifact, 0o644))

	svc, repo := setupPluginServiceWithOptions(t, plugin.NoopRuntimeManager{}, plugin.NewMemoryHookDispatcher(4),
		plugin.WithBundled(dir, true, false, "local"),
	)
	require.NoError(t, svc.SeedBundled(context.Background()))
	rows, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.NotNil(t, rows[0].PermissionsAckAt)
}

func TestSeedBundled_StrictModeWithoutSignature(t *testing.T) {
	artifact := zipArtifact(t, manifestJSON("gosite/demo", "1.0.0", 0), nil)
	dir := writeBundledDir(t, "", "demo.zip", artifact)
	svc, repo := setupPluginServiceWithOptions(t, plugin.NoopRuntimeManager{}, plugin.NewMemoryHookDispatcher(4),
		plugin.WithBundled(dir, true, false, "production"),
		plugin.WithAllowUnsigned(false),
	)
	require.NoError(t, svc.SeedBundled(context.Background()))
	rows, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
}

func TestSeedBundled_IgnoresMinGoSiteVersionForBundled(t *testing.T) {
	manifest := strings.Replace(manifestJSON("gosite/demo", "1.0.0", 0), `"minGoSiteVersion":"0.1.0"`, `"minGoSiteVersion":"9.9.0"`, 1)
	artifact := zipArtifact(t, manifest, nil)
	dir := writeBundledDir(t, "", "demo.zip", artifact)
	svc, repo := setupPluginServiceWithOptions(t, plugin.NoopRuntimeManager{}, plugin.NewMemoryHookDispatcher(4),
		plugin.WithBundled(dir, true, false, "production"),
		plugin.WithHostVersion("1.0.0"),
	)
	require.NoError(t, svc.SeedBundled(context.Background()))
	rows, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, sqlite.PluginStateInstalled, rows[0].State)
}
