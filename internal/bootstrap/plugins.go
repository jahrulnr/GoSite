package bootstrap

import (
	"context"
	"database/sql"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	pluginsvc "github.com/jahrulnr/gosite/internal/service/plugin"
)

func seedBundledPlugins(ctx context.Context, cfg config.Config, db *sql.DB) error {
	if !cfg.PluginBundledEnabled {
		return nil
	}
	repo := sqlite.NewPluginRepository(db)
	svc := pluginsvc.NewService(
		repo,
		cfg.Storage,
		pluginsvc.NoopRuntimeManager{},
		pluginsvc.NoopHookDispatcher{},
		pluginsvc.WithAllowUnsigned(cfg.PluginAllowUnsigned),
		pluginsvc.WithKeyringPath(cfg.PluginKeyringPath),
		pluginsvc.WithHostVersion(cfg.AppVersion),
		pluginsvc.WithBundled(cfg.PluginBundledPath, cfg.PluginBundledEnabled, cfg.PluginBundledAutoEnable, cfg.AppEnv),
	)
	return svc.SeedBundled(ctx)
}
