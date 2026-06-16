package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/buildinfo"
)

// Config holds runtime settings loaded from environment variables.
type Config struct {
	AppEnv     string
	AppVersion string
	Storage    string
	WebPath    string
	Database   string

	AuthEnable bool
	AuthUser   string
	AuthPass   string

	EnableLockscreen bool
	LockAfter        time.Duration

	MailNotification bool

	FilesAllowExecute bool

	LogEventsRetentionDays int
	AuditRetentionDays     int

	PluginAllowUnsigned      bool
	PluginKeyringPath        string
	PluginMaxConcurrentHooks int
	PluginHookTimeout        time.Duration
	PluginHealthInterval     time.Duration
	PluginRestartMaxAttempts int
	PluginRestartWindow      time.Duration
	PluginRestartBackoffMin  time.Duration
	PluginRestartBackoffMax  time.Duration
	PluginWebhookSecret      string

	ListenAddr string
	TLSCert    string
	TLSKey     string

	TemplatesDir   string
	MigrationsDir  string
	EtcDir         string
	LetsEncryptDir string

	FEEmbed             bool
	SessionCookieSecure bool
	TLSEnable           bool
	CORSOrigins         []string

	// Terminal (xterm.js floating window).
	TerminalStickyTTL  time.Duration
	TerminalDumpDir    string
	TerminalDumpMax    int64
	TerminalPerUserMax int
}

// LogsDir returns the centralized nginx and app log directory.
func (c Config) LogsDir() string {
	return filepath.Join(c.Storage, "logs")
}

// StorageLayout returns persistent directories created during bootstrap.
func (c Config) StorageLayout() []string {
	return []string{
		c.LogsDir(),
		filepath.Join(c.Storage, "www"),
		filepath.Join(c.Storage, "webconfig", "site.d"),
		filepath.Join(c.Storage, "webconfig", "active.d"),
		filepath.Join(c.Storage, "webconfig", "ssl", "live", "default"),
		filepath.Join(c.Storage, "mount-secrets"),
		filepath.Join(c.Storage, "plugins"),
	}
}

// Load reads configuration from environment variables with production-friendly defaults.
func Load() Config {
	storage := envOr("STORAGE_PATH", "/storage")
	dbPath := envOr("DB_DATABASE", filepath.Join(storage, "db.sqlite"))
	appEnv := envOr("APP_ENV", "production")

	return Config{
		AppEnv:     appEnv,
		AppVersion: envOr("APP_VERSION", buildinfo.Version),
		Storage:    storage,
		WebPath:    envOr("WEB_PATH", "/www"),
		Database:   dbPath,

		AuthEnable: envBool("AUTH_ENABLE", true),
		AuthUser:   envOr("AUTH_USER", "admin"),
		AuthPass:   envOr("AUTH_PASS", "admin"),

		EnableLockscreen: envBool("ENABLE_LOCKSCREEN", false),
		LockAfter:        time.Duration(envInt("LOCK_AFTER", 300)) * time.Second,

		MailNotification: envBool("MAIL_NOTIFICATION", true),

		FilesAllowExecute: envBool("FILES_ALLOW_EXECUTE", false),

		LogEventsRetentionDays: envInt("LOG_EVENTS_RETENTION_DAYS", 14),
		AuditRetentionDays:     envInt("AUDIT_RETENTION_DAYS", 90),

		PluginAllowUnsigned:      envBool("PLUGIN_ALLOW_UNSIGNED", appEnv != "production"),
		PluginKeyringPath:        envOr("PLUGIN_KEYRING_PATH", filepath.Join(storage, "plugins", "keyring.json")),
		PluginHookTimeout:        envDuration("PLUGIN_HOOK_TIMEOUT", 5*time.Second),
		PluginMaxConcurrentHooks: envInt("PLUGIN_MAX_CONCURRENT_HOOKS", 10),
		PluginHealthInterval:     envDuration("PLUGIN_HEALTH_CHECK_INTERVAL", 30*time.Second),
		PluginRestartMaxAttempts: envInt("PLUGIN_RESTART_MAX_ATTEMPTS", 5),
		PluginRestartWindow:      envDuration("PLUGIN_RESTART_WINDOW", 10*time.Minute),
		PluginRestartBackoffMin:  envDuration("PLUGIN_RESTART_BACKOFF_INITIAL", 1*time.Second),
		PluginRestartBackoffMax:  envDuration("PLUGIN_RESTART_BACKOFF_CAP", 2*time.Minute),
		PluginWebhookSecret:      envOr("PLUGIN_WEBHOOK_SECRET", ""),

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

		TerminalStickyTTL:  envDuration("TERMINAL_STICKY_TTL", 12*time.Hour),
		TerminalDumpDir:    envOr("TERMINAL_DUMP_DIR", "/tmp"),
		TerminalDumpMax:    int64(envInt("TERMINAL_DUMP_MAX", 256*1024)),
		TerminalPerUserMax: envInt("TERMINAL_PER_USER_MAX", 8),
	}
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return parsed
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
