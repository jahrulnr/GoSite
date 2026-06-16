package secrets_test

import (
	"testing"

	"github.com/jahrulnr/gosite/pkg/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCipherRoundTrip(t *testing.T) {
	t.Parallel()
	cipher, err := secrets.NewCipher(secrets.DerivedSource{Seed: "test"})
	require.NoError(t, err)

	plaintext := []byte(`{"api_key":"super-secret"}`)
	envelope, err := cipher.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, envelope)

	got, err := cipher.Decrypt(envelope)
	require.NoError(t, err)
	assert.Equal(t, plaintext, got)
}

func TestCipherRejectsTamperedEnvelope(t *testing.T) {
	t.Parallel()
	cipher, err := secrets.NewCipher(secrets.DerivedSource{Seed: "test"})
	require.NoError(t, err)

	envelope, err := cipher.Encrypt([]byte("hello"))
	require.NoError(t, err)
	envelope[len(envelope)-1] ^= 0xFF
	_, err = cipher.Decrypt(envelope)
	require.Error(t, err)
}

func TestEnvSourceRejectsMissingKey(t *testing.T) {
	t.Setenv("PLUGIN_CONFIG_KEY", "")
	_, err := secrets.NewCipher(secrets.NewEnvSource())
	require.Error(t, err)
}
