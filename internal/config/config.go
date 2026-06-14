package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings loaded from environment variables.
type Config struct {
	AppEnv   string
	Storage  string
	WebPath  string
	Database string

	AuthEnable bool
	AuthUser   string
	AuthPass   string

	EnableLockscreen bool
	LockAfter        time.Duration

	MailNotification bool

	FilesAllowExecute bool

	LogEventsRetentionDays int
	AuditRetentionDays     int

	ListenAddr string
	TLSCert    string
	TLSKey     string

	TemplatesDir   string
	MigrationsDir  string
	EtcDir         string
	LetsEncryptDir string

	FEEmbed              bool
	SessionCookieSecure  bool
	TLSEnable            bool
	CORSOrigins          []string
}

// StorageLayout returns persistent directories created during bootstrap.
func (c Config) StorageLayout() []string {
	return []string{
		filepath.Join(c.Storage, "laravel", "logs"),
		filepath.Join(c.Storage, "www"),
		filepath.Join(c.Storage, "webconfig", "site.d"),
		filepath.Join(c.Storage, "webconfig", "active.d"),
		filepath.Join(c.Storage, "webconfig", "ssl", "live", "default"),
	}
}

// Load reads configuration from environment variables with production-friendly defaults.
func Load() Config {
	storage := envOr("STORAGE_PATH", "/storage")
	dbPath := envOr("DB_DATABASE", filepath.Join(storage, "db.sqlite"))

	return Config{
		AppEnv:   envOr("APP_ENV", "production"),
		Storage:  storage,
		WebPath:  envOr("WEB_PATH", "/www"),
		Database: dbPath,

		AuthEnable: envBool("AUTH_ENABLE", true),
		AuthUser:   envOr("AUTH_USER", "admin"),
		AuthPass:   envOr("AUTH_PASS", "admin"),

		EnableLockscreen: envBool("ENABLE_LOCKSCREEN", false),
		LockAfter:        time.Duration(envInt("LOCK_AFTER", 300)) * time.Second,

		MailNotification: envBool("MAIL_NOTIFICATION", true),

		FilesAllowExecute: envBool("FILES_ALLOW_EXECUTE", false),

		LogEventsRetentionDays: envInt("LOG_EVENTS_RETENTION_DAYS", 14),
		AuditRetentionDays:     envInt("AUDIT_RETENTION_DAYS", 90),

		ListenAddr: envOr("LISTEN_ADDR", ":8080"),
		TLSCert:    envOr("TLS_CERT", filepath.Join(storage, "webconfig/ssl/live/default/cert.pem")),
		TLSKey:     envOr("TLS_KEY", filepath.Join(storage, "webconfig/ssl/live/default/key.pem")),

		TemplatesDir:   envOr("TEMPLATES_DIR", "/var/setup"),
		MigrationsDir:  envOr("MIGRATIONS_DIR", "migrations"),
		EtcDir:         envOr("ETC_DIR", "/etc"),
		LetsEncryptDir: envOr("LETSENCRYPT_DIR", "/etc/letsencrypt"),

		FEEmbed:             envBool("FE_EMBED", false),
		SessionCookieSecure: envBool("SESSION_COOKIE_SECURE", true),
		TLSEnable:           envBool("TLS_ENABLE", true),
		CORSOrigins:         splitCSV(envOr("CORS_ORIGINS", "")),
	}
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
