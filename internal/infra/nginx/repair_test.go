package nginx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNginxError(t *testing.T) {
	t.Parallel()

	msg := `nginx: [emerg] cannot load certificate "/storage/webconfig/ssl/live/example.com/cert.pem": BIO_new_file() failed (2: No such file or directory) in /storage/webconfig/site.d/example.com.conf:24`
	parsed, ok := parseNginxError(msg)
	require.True(t, ok)
	assert.Equal(t, "/storage/webconfig/site.d/example.com.conf", parsed.File)
	assert.Equal(t, 24, parsed.Line)
}

func TestCommentOutLine(t *testing.T) {
	t.Parallel()

	lines := []string{"server {", "\tlisten 443 ssl;", "}"}
	updated, ok := commentOutLine(lines, 2)
	require.True(t, ok)
	assert.Contains(t, updated[1], "# gosite-repair:")
	assert.Contains(t, updated[1], "listen 443 ssl;")
}

func TestRepairMissingCertificate_ReplacesDirective(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(cert, []byte("cert"), 0o644))
	require.NoError(t, os.WriteFile(key, []byte("key"), 0o600))

	lines := []string{
		"server {",
		"    listen 443 ssl;",
		"    ssl_certificate /missing/cert.pem;",
		"    ssl_certificate_key /missing/key.pem;",
		"}",
	}
	cfg := RepairConfig{DefaultCert: cert, DefaultKey: key}
	updated, ok := repairMissingCertificate(lines, 3, cfg, `cannot load certificate "/missing/cert.pem"`)
	require.True(t, ok)
	assert.Contains(t, updated[2], cert)
	assert.Contains(t, updated[3], key)
}

func TestRepairUndefinedSSL_InsertsDefaults(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(cert, []byte("cert"), 0o644))
	require.NoError(t, os.WriteFile(key, []byte("key"), 0o600))

	lines := []string{
		"server {",
		"    listen 443 ssl;",
		"    server_name example.com;",
		"}",
	}
	cfg := RepairConfig{DefaultCert: cert, DefaultKey: key}
	updated, ok := repairUndefinedSSL(lines, 2, cfg)
	require.True(t, ok)
	body := strings.Join(updated, "\n")
	assert.Contains(t, body, "ssl_certificate "+cert)
	assert.Contains(t, body, "ssl_certificate_key "+key)
}

func TestIsRepairAllowed(t *testing.T) {
	t.Parallel()
	prefixes := []string{"/storage/webconfig/site.d"}
	assert.True(t, isRepairAllowed("/storage/webconfig/site.d/foo.conf", prefixes))
	assert.False(t, isRepairAllowed("/etc/passwd", prefixes))
}

func TestApplyRepair_CommentsUnknownDirective(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	conf := filepath.Join(dir, "site.conf")
	require.NoError(t, os.WriteFile(conf, []byte("server {\n\tbad_directive on;\n}\n"), 0o644))

	cfg := RepairConfig{AllowPrefixes: []string{dir}}
	parsed := parsedNginxError{
		Message: `nginx: [emerg] unknown directive "bad_directive" in ` + conf + `:2`,
		File:    conf,
		Line:    2,
	}
	action, applied, err := applyRepair(parsed, cfg)
	require.NoError(t, err)
	require.True(t, applied)
	assert.Equal(t, "comment out line", action.Fix)

	data, err := os.ReadFile(conf)
	require.NoError(t, err)
	assert.Contains(t, string(data), "# gosite-repair: bad_directive on;")
}
