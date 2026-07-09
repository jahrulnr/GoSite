package bundled_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/service/plugin/bundled"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBundledService_Enabled(t *testing.T) {
	assert.True(t, bundled.NewService(t.TempDir()).Enabled())
	assert.False(t, bundled.NewService("").Enabled())
}

func TestBundledService_LoadIndexInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundled-plugins.json"), []byte("{"), 0o644))
	_, err := bundled.NewService(dir).LoadIndex()
	require.Error(t, err)
}

func TestBundledService_LoadArtifactWithoutResolveDir(t *testing.T) {
	svc := bundled.NewService("")
	_, err := svc.LoadArtifact(bundled.Entry{Artifact: "demo.zip"})
	require.ErrorIs(t, err, bundled.ErrArtifactsUnavailable)
}

func TestBundledService_LoadEmbeddedIndex(t *testing.T) {
	svc := bundled.NewService("")
	index, err := svc.LoadIndex()
	require.NoError(t, err)
	require.NotEmpty(t, index.Plugins)
	assert.Equal(t, "gosite/mcp", index.Plugins[0].PluginID)
}

func TestBundledService_LoadArtifactFromDir(t *testing.T) {
	dir := t.TempDir()
	index := `{"apiVersion":"gosite-plugin-bundled/1","plugins":[{"plugin_id":"acme/demo","artifact":"demo.zip","restorable":true}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bundled-plugins.json"), []byte(index), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "demo.zip"), []byte("zip-bytes"), 0o644))

	svc := bundled.NewService(dir)
	entry, err := svc.Entry("acme/demo")
	require.NoError(t, err)
	data, err := svc.LoadArtifact(entry)
	require.NoError(t, err)
	assert.Equal(t, []byte("zip-bytes"), data)
}

func TestBundledService_LoadArtifactMissing(t *testing.T) {
	svc := bundled.NewService(t.TempDir())
	_, err := svc.LoadArtifact(bundled.Entry{Artifact: "missing.zip"})
	require.ErrorIs(t, err, bundled.ErrArtifactsUnavailable)
}

func TestBundledService_EntryNotFound(t *testing.T) {
	svc := bundled.NewService("")
	_, err := svc.Entry("missing/plugin")
	require.ErrorIs(t, err, bundled.ErrNotFound)
}
