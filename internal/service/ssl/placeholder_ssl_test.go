package ssl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsPlaceholderSSL(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	assert.False(t, isPlaceholderSSL(filepath.Join(dir, "missing")))

	live := filepath.Join(dir, "live")
	require.NoError(t, os.MkdirAll(live, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(live, "cert.pem"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(live, "key.pem"), []byte("y"), 0o600))
	assert.True(t, isPlaceholderSSL(live))

	require.NoError(t, os.Symlink("../../archive/example/fullchain1.pem", filepath.Join(live, "fullchain.pem")))
	assert.False(t, isPlaceholderSSL(live))
}

func TestClearPlaceholderSSL(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	domain := "example.com"
	liveDir := filepath.Join(root, "webconfig/ssl/live", domain)
	require.NoError(t, os.MkdirAll(liveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(liveDir, "cert.pem"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(liveDir, "key.pem"), []byte("y"), 0o600))

	require.NoError(t, clearPlaceholderSSL(root, domain))
	_, err := os.Stat(liveDir)
	assert.True(t, os.IsNotExist(err))
}

func TestSSLPathsEqual(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "cert.pem")
	b := filepath.Join(dir, "cert.pem")
	assert.True(t, sslPathsEqual(a, b))
}
