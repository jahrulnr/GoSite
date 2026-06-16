package plugin

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyringAddAndRevoke(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "keyring.json")

	err := AddKeyringEntry(context.Background(), path, TrustedKey{
		Vendor:    "acme",
		KeyID:     "vendor-1",
		PublicKey: "BASE64==",
	})
	require.NoError(t, err)

	keys, err := LoadKeyring(context.Background(), path)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "acme", keys[0].Vendor)

	err = RevokeKeyringEntry(context.Background(), path, "acme", "vendor-1")
	require.NoError(t, err)
	keys, err = LoadKeyring(context.Background(), path)
	require.NoError(t, err)
	assert.NotEmpty(t, keys[0].RevokedAt)
}
