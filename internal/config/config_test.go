package config_test

import (
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_LoadDefaults(t *testing.T) {
	t.Setenv("STORAGE_PATH", "")
	t.Setenv("DB_DATABASE", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("WEB_PATH", "")
	t.Setenv("AUTH_ENABLE", "")
	t.Setenv("AUTH_USER", "")
	t.Setenv("AUTH_PASS", "")
	t.Setenv("ENABLE_LOCKSCREEN", "")
	t.Setenv("LOCK_AFTER", "")
	t.Setenv("MAIL_NOTIFICATION", "")
	t.Setenv("FILES_ALLOW_EXECUTE", "")
	t.Setenv("LOG_EVENTS_RETENTION_DAYS", "")
	t.Setenv("AUDIT_RETENTION_DAYS", "")
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("TLS_CERT", "")
	t.Setenv("TLS_KEY", "")
	t.Setenv("TEMPLATES_DIR", "")
	t.Setenv("MIGRATIONS_DIR", "")
	t.Setenv("ETC_DIR", "")
	t.Setenv("LETSENCRYPT_DIR", "")
	t.Setenv("FE_EMBED", "")
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("TLS_ENABLE", "")
	t.Setenv("CORS_ORIGINS", "")

	cfg := config.Load()

	require.Equal(t, "production", cfg.AppEnv)
	assert.Equal(t, "/storage", cfg.Storage)
	assert.Equal(t, "/www", cfg.WebPath)
	assert.Equal(t, "/storage/db.sqlite", cfg.Database)

	assert.True(t, cfg.AuthEnable)
	assert.Equal(t, "admin", cfg.AuthUser)
	assert.Equal(t, "admin", cfg.AuthPass)

	assert.False(t, cfg.EnableLockscreen)
	assert.Equal(t, 300*time.Second, cfg.LockAfter)

	assert.True(t, cfg.MailNotification)
	assert.False(t, cfg.FilesAllowExecute)

	assert.Equal(t, 14, cfg.LogEventsRetentionDays)
	assert.Equal(t, 90, cfg.AuditRetentionDays)

	assert.Equal(t, ":8080", cfg.ListenAddr)
	assert.Equal(t, "/storage/webconfig/ssl/live/default/cert.pem", cfg.TLSCert)
	assert.Equal(t, "/storage/webconfig/ssl/live/default/key.pem", cfg.TLSKey)
	assert.Equal(t, "/var/setup", cfg.TemplatesDir)
	assert.Equal(t, "migrations", cfg.MigrationsDir)
	assert.Equal(t, "/etc", cfg.EtcDir)
	assert.Equal(t, "/etc/letsencrypt", cfg.LetsEncryptDir)

	assert.False(t, cfg.FEEmbed)
	assert.True(t, cfg.SessionCookieSecure)
	assert.True(t, cfg.TLSEnable)
	assert.Nil(t, cfg.CORSOrigins)
}

func TestConfig_FrontendOverrides(t *testing.T) {
	t.Setenv("FE_EMBED", "false")
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	t.Setenv("CORS_ORIGINS", "http://localhost:5173, https://panel.example.com")

	cfg := config.Load()

	assert.False(t, cfg.FEEmbed)
	assert.False(t, cfg.SessionCookieSecure)
	assert.Equal(t, []string{"http://localhost:5173", "https://panel.example.com"}, cfg.CORSOrigins)
}
