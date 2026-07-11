package http

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/delivery/http/frontend"
	"github.com/jahrulnr/gosite/internal/delivery/http/handler"
	"github.com/jahrulnr/gosite/internal/delivery/http/middleware"
	"github.com/jahrulnr/gosite/internal/infra/commander"
	dockerinfra "github.com/jahrulnr/gosite/internal/infra/docker"
	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/observability/grafanalite"
	"github.com/jahrulnr/gosite/internal/observability/nginxlite"
	"github.com/jahrulnr/gosite/internal/observability/splunklite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	cronsvc "github.com/jahrulnr/gosite/internal/service/cron"
	"github.com/jahrulnr/gosite/internal/service/database"
	dockersvc "github.com/jahrulnr/gosite/internal/service/docker"
	filessvc "github.com/jahrulnr/gosite/internal/service/files"
	"github.com/jahrulnr/gosite/internal/service/logs"
	mountsvc "github.com/jahrulnr/gosite/internal/service/mount"
	pluginsvc "github.com/jahrulnr/gosite/internal/service/plugin"
	plugincatalog "github.com/jahrulnr/gosite/internal/service/plugin/catalog"
	"github.com/jahrulnr/gosite/internal/service/plugin/hookbus"
	pluginremote "github.com/jahrulnr/gosite/internal/service/plugin/remote"
	"github.com/jahrulnr/gosite/internal/service/settings"
	"github.com/jahrulnr/gosite/internal/service/ssl"
	"github.com/jahrulnr/gosite/internal/service/system"
	"github.com/jahrulnr/gosite/internal/service/uimeta"
	"github.com/jahrulnr/gosite/internal/service/website"
	"github.com/jahrulnr/gosite/internal/terminal"
	"github.com/jahrulnr/gosite/pkg/secrets"
)

// NewRouter wires the Gin engine with API routes.
func NewRouter(cfg config.Config, db *sql.DB) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	sessionRepo := sqlite.NewSessionRepository(db)
	sessionPersister := auth.NewSQLitePersister(sessionRepo)
	sessions := auth.NewStoreWithOptions(0, cfg.SessionCookieSecure, sessionPersister)
	users := sqlite.NewUserRepository(db)
	lockscreen := auth.NewLockscreen()
	authSvc := auth.NewService(users, sessions, auth.WithLockscreen(lockscreen))
	go purgeExpiredSessions(sessionRepo)

	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authSvc, sessions, auth.LoginMetadataFromConfig(
		cfg.EnableLockscreen,
		cfg.AuthEnable,
		int(cfg.LockAfter.Seconds()),
		cfg.WebPath,
		cfg.Storage,
	))

	cmd := commander.NewExecRunner()
	auditRepo := sqlite.NewAuditRepository(db)
	pluginRepo := sqlite.NewPluginRepository(db)
	pluginRuntime := pluginsvc.NewGoPluginRuntimeManager()
	pluginDispatcher := hookbus.New(hookbus.Config{
		MaxConcurrentHooks: cfg.PluginMaxConcurrentHooks,
		HookTimeout:        cfg.PluginHookTimeout,
		Audit:              splunklite.NewAuditWriter(auditRepo),
		Caller:             pluginsvc.NewHookCallerAdapter(pluginRuntime),
	})
	ngx := nginx.NewServiceFromConfig(cfg, cmd, nginx.WithHookBus(pluginDispatcher))
	websiteRepo := sqlite.NewWebsiteRepository(db)
	jobRepo := sqlite.NewJobRepository(db)
	worker := job.NewWorker(jobRepo, cmd, 32, job.WithHookBus(pluginDispatcher))
	worker.Start(context.Background(), 2)

	websiteSvc := website.NewService(websiteRepo, ngx, cfg.WebPath, website.WithHookBus(pluginDispatcher))
	sslSvc := ssl.NewService(websiteRepo, jobRepo, ngx, worker, ssl.WithHookBus(pluginDispatcher))

	websiteHandler := handler.NewWebsiteHandler(websiteSvc)
	nginxHandler := handler.NewNginxHandler(ngx)
	sslHandler := handler.NewSSLHandler(sslSvc, worker)

	logRepo := sqlite.NewLogEventRepository(db)
	metricsRepo := sqlite.NewTrafficMetricsRepository(db)
	nginxStatusRepo := sqlite.NewNginxStatusRepository(db)
	nginxVTSRepo := sqlite.NewNginxVTSRepository(db)
	savedRepo := sqlite.NewSavedQueryRepository(db)
	splunkSvc := splunklite.NewService(auditRepo, jobRepo, logRepo, savedRepo, cfg.AuditRetentionDays, cfg.LogEventsRetentionDays)
	grafanaSvc := grafanalite.NewService(metricsRepo)
	nginxMetricsSvc := nginxlite.NewService(nginxStatusRepo, nginxVTSRepo, cfg.NginxVTSStatusURL)
	logDir := cfg.LogsDir()
	logsSvc := logs.NewService(logDir, websiteRepo)
	queryMetaSvc := splunklite.NewMetaService(logsSvc, logDir)
	logIngestor := splunklite.NewLogIngestor(logRepo, logDir)
	splunkSvc.SetIngestor(logIngestor)
	obsHandler := handler.NewObservabilityHandler(splunkSvc, queryMetaSvc, logIngestor, grafanaSvc, nginxMetricsSvc)

	systemSvc := system.NewService(logDir, nil, system.CommandAdapter{Runner: cmd})
	settingsSvc := settings.NewService(users)
	databaseSvc := database.NewService(db, cfg.Database)
	uimetaSvc := uimeta.NewService(cfg)

	systemHandler := handler.NewSystemHandler(systemSvc)
	settingsHandler := handler.NewSettingsHandler(settingsSvc, authSvc)
	logsHandler := handler.NewLogsHandler(logsSvc)
	databaseHandler := handler.NewDatabaseHandler(databaseSvc)
	uimetaHandler := handler.NewUIMetaHandler(uimetaSvc)
	dashboardHandler := handler.NewDashboardHandler(systemSvc, sslSvc, splunkSvc, grafanaSvc, nginxMetricsSvc)

	dockerClient, err := dockerinfra.NewClient("")
	var dockerSvc contracts.DockerClient
	if err != nil {
		dockerSvc = dockerinfra.NoopClient{}
	} else {
		dockerSvc = dockerClient
	}
	dockerService := dockersvc.NewService(dockerSvc, dockersvc.WithHookBus(pluginDispatcher))
	dockerHandler := handler.NewDockerHandler(dockerService)

	fileRoots := []string{cfg.WebPath, cfg.Storage, "/tmp", "/"}
	filesSvc := filessvc.NewService(fileRoots, cfg.FilesAllowExecute, cmd)
	filesHandler := handler.NewFilesHandler(filesSvc)

	fstabPath := filepath.Join(cfg.EtcDir, "fstab")
	secretsDir := filepath.Join(cfg.Storage, "mount-secrets")
	mountSvc := mountsvc.NewService(fstabPath, secretsDir, cmd)
	mountHandler := handler.NewMountHandler(mountSvc)

	cronRepo := sqlite.NewCronJobRepository(db)
	cronSvc := cronsvc.NewService(cronRepo, jobRepo, worker, cronsvc.WithHookBus(pluginDispatcher))
	cronHandler := handler.NewCronHandler(cronSvc, worker)

	pluginConfigRepo := sqlite.NewPluginConfigRepository(db)
	pluginAccessTokenRepo := sqlite.NewPluginAccessTokenRepository(db)
	pluginCipher, err := secrets.NewCipher(secrets.NewDerivedSource(cfg.Storage))
	if err != nil {
		slog.Warn("plugin secret cipher disabled", "err", err)
	}
	pluginConfigSvc := pluginsvc.NewConfigService(
		pluginConfigRepo,
		pluginsvc.WithCipher(pluginCipher),
	)
	integrationTokenSvc := pluginsvc.NewIntegrationTokenService(pluginAccessTokenRepo, pluginRepo, splunklite.NewAuditWriter(auditRepo))
	pluginSvc := pluginsvc.NewService(
		pluginRepo,
		cfg.Storage,
		pluginRuntime,
		pluginDispatcher,
		pluginsvc.WithAllowUnsigned(cfg.PluginAllowUnsigned),
		pluginsvc.WithKeyringPath(cfg.PluginKeyringPath),
		pluginsvc.WithHostVersion(cfg.AppVersion),
		pluginsvc.WithConfigRepo(pluginConfigRepo),
		pluginsvc.WithIntegrationTokens(integrationTokenSvc),
		pluginsvc.WithBundled(cfg.PluginBundledPath, cfg.PluginBundledEnabled, cfg.PluginBundledAutoEnable, cfg.AppEnv),
	)
	if err := pluginSvc.Reconcile(context.Background()); err != nil {
		slog.Warn("plugin reconcile failed", "err", err)
	}
	pluginRemoteCfg := pluginremote.ConfigFromApp(cfg)
	pluginRemoteSvc := pluginremote.NewService(pluginRemoteCfg)
	pluginCatalogSvc := plugincatalog.NewService(cfg.PluginCatalogPath)
	pluginHandler := handler.NewPluginHandler(pluginSvc, pluginRemoteSvc, pluginRemoteCfg, pluginCatalogSvc)
	pluginConfigHandler := handler.NewConfigHandler(pluginConfigSvc)
	pluginKeyringHandler := handler.NewKeyringHandler(cfg.PluginKeyringPath)
	integrationTokenHandler := handler.NewIntegrationTokenHandler(integrationTokenSvc, authSvc, users)

	supervisor := pluginsvc.NewHealthSupervisor(
		pluginRepo,
		pluginRuntime,
		pluginRuntime,
		pluginsvc.WithSupervisorInterval(cfg.PluginHealthInterval),
		pluginsvc.WithSupervisorMaxAttempts(cfg.PluginRestartMaxAttempts),
		pluginsvc.WithSupervisorWindow(cfg.PluginRestartWindow),
		pluginsvc.WithSupervisorBackoff(cfg.PluginRestartBackoffMin, cfg.PluginRestartBackoffMax),
	)
	supervisorCtx, supervisorCancel := context.WithCancel(context.Background())
	go supervisor.Run(supervisorCtx)
	_ = supervisorCancel

	terminalHub := terminal.NewHub(terminal.HubConfig{
		StickyTTL:    cfg.TerminalStickyTTL,
		DumpDir:      cfg.TerminalDumpDir,
		DumpMax:      cfg.TerminalDumpMax,
		PerUser:      cfg.TerminalPerUserMax,
		DefaultShell: "/bin/bash",
		DefaultCwd:   cfg.Storage,
		DefaultEnv:   []string{"TERM=xterm-256color", "COLORTERM=truecolor", "LANG=C.UTF-8", "HOME=" + cfg.Storage},
	})
	if err := terminalHub.EnsureDumpDir(); err != nil {
		slog.Warn("terminal dump dir could not be created", "err", err, "dir", cfg.TerminalDumpDir)
	}
	terminalHubCtx, terminalCancel := context.WithCancel(context.Background())
	defer terminalCancel()
	terminalHub.RunSweeper(terminalHubCtx, time.Minute)
	terminalHandler := handler.NewTerminalHandler(terminalHub, splunklite.NewAuditWriter(auditRepo), authSvc)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.CORS(cfg.CORSOrigins))

	engine.GET("/health", healthHandler.Health)

	api := engine.Group("/api/v1")
	api.Use(middleware.BasicAuth(cfg))

	authGroup := api.Group("/auth")
	{
		authGroup.GET("/login", authHandler.LoginMetadata)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/logout", middleware.RequireSession(authSvc), authHandler.Logout)
		authGroup.GET("/me", middleware.RequireSession(authSvc), authHandler.Me)
		authGroup.GET("/lockscreen", middleware.RequireSession(authSvc), authHandler.Lockscreen)
		authGroup.POST("/lock", middleware.RequireSession(authSvc), authHandler.Lock)
		authGroup.POST("/unlock", middleware.RequireSession(authSvc), authHandler.Unlock)
	}

	protected := api.Group("")
	protected.Use(middleware.RequireSessionOrAccessToken(authSvc, integrationTokenSvc))
	registerWebsiteRoutes(protected, websiteHandler, integrationTokenSvc)
	registerNginxRoutes(protected, nginxHandler, integrationTokenSvc)
	registerSSLRoutes(protected, sslHandler)
	registerDockerRoutes(protected, dockerHandler, integrationTokenSvc)
	registerFilesRoutes(protected, filesHandler)
	registerMountRoutes(protected, mountHandler)
	registerCronRoutes(protected, cronHandler)
	registerObservabilityRoutes(protected, obsHandler)
	registerDashboardRoutes(protected, dashboardHandler, integrationTokenSvc)
	registerSystemRoutes(protected, systemHandler, integrationTokenSvc)
	sessionOnly := protected.Group("")
	sessionOnly.Use(middleware.RequireSessionOnly())
	registerSettingsRoutes(sessionOnly, settingsHandler)
	registerLogsRoutes(protected, logsHandler)
	registerDatabaseRoutes(protected, databaseHandler)
	registerUIMetaRoutes(protected, uimetaHandler)
	registerTerminalRoutes(sessionOnly, terminalHandler)
	registerPluginPublicRoutes(protected, pluginHandler)
	registerPluginRoutes(sessionOnly, pluginHandler, pluginConfigHandler, pluginKeyringHandler, integrationTokenHandler)

	api.GET("/integration-tokens/self",
		middleware.RequireAccessTokenOnly(integrationTokenSvc),
		gin.WrapF(integrationTokenHandler.Self),
	)

	if cfg.FEEmbed {
		frontend.Register(engine, frontend.DistFS)
	}

	return engine
}

func registerDashboardRoutes(api *gin.RouterGroup, h *handler.DashboardHandler, tokens *pluginsvc.IntegrationTokenService) {
	_ = tokens
	api.GET("/dashboard", middleware.RequireScope("dashboard:read"), gin.WrapF(h.Get))
}

func registerSystemRoutes(api *gin.RouterGroup, h *handler.SystemHandler, tokens *pluginsvc.IntegrationTokenService) {
	api.GET("/system/info", middleware.RequireScope("system:read"), gin.WrapF(h.Info))
	api.GET("/system/network", middleware.RequireScope("system:read"), gin.WrapF(h.Network))
	api.GET("/system/disk-io", middleware.RequireScope("system:read"), gin.WrapF(h.DiskIO))
	api.GET("/system/nginx-traffic", middleware.RequireScope("system:read"), gin.WrapF(h.NginxTraffic))
	_ = tokens
}

func registerSettingsRoutes(api *gin.RouterGroup, h *handler.SettingsHandler) {
	api.PUT("/settings/profile", gin.WrapF(h.UpdateProfile))
}

func registerLogsRoutes(api *gin.RouterGroup, h *handler.LogsHandler) {
	api.GET("/logs/sites", middleware.RequireScope("query:read"), gin.WrapF(h.ListSites))
	api.GET("/logs", middleware.RequireScope("query:read"), gin.WrapF(h.Tail))
}

func registerDatabaseRoutes(api *gin.RouterGroup, h *handler.DatabaseHandler) {
	api.GET("/database/tables", middleware.RequireScope("query:read"), gin.WrapF(h.ListTables))
	api.GET("/database/tables/:name", middleware.RequireScope("query:read"), gin.WrapF(h.GetTable))
}

func registerUIMetaRoutes(api *gin.RouterGroup, h *handler.UIMetaHandler) {
	api.GET("/ui/meta", middleware.RequireScope("ui:read"), gin.WrapF(h.Get))
}

// purgeExpiredSessions removes stale session rows on a 15-minute cadence so
// the sessions table never grows unbounded.
func purgeExpiredSessions(repo *sqlite.SessionRepository) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if n, err := repo.PurgeExpired(ctx); err != nil {
			slog.Warn("purge expired sessions failed", "err", err)
		} else if n > 0 {
			slog.Info("purged expired sessions", "count", n)
		}
		cancel()
	}
}

func registerWebsiteRoutes(api *gin.RouterGroup, h *handler.WebsiteHandler, tokens *pluginsvc.IntegrationTokenService) {
	api.GET("/websites", middleware.RequireScope("websites:read"), gin.WrapF(h.List))
	api.POST("/websites", middleware.RequireScope("websites:write"), gin.WrapF(h.Create))
	api.POST("/websites/validate", middleware.RequireScope("websites:read"), gin.WrapF(h.Validate))
	api.GET("/websites/:id", middleware.RequireScope("websites:read"), gin.WrapF(h.Get))
	api.PUT("/websites/:id", middleware.RequireScope("websites:write"), gin.WrapF(h.Update))
	api.DELETE("/websites/:id", middleware.RequireScope("websites:write"), gin.WrapF(h.Delete))
	api.PATCH("/websites/:id/toggle", middleware.RequireScope("websites:write"), gin.WrapF(h.Toggle))
	api.GET("/websites/:id/nginx-config", middleware.RequireScope("websites:read"), gin.WrapF(h.GetNginxConfig))
	api.PUT("/websites/:id/nginx-config", middleware.RequireScope("websites:write"), gin.WrapF(h.UpdateNginxConfig))
	api.POST("/websites/:id/nginx-config/test", middleware.RequireScope("websites:read"), gin.WrapF(h.TestNginxConfig))
	_ = tokens
}

func registerNginxRoutes(api *gin.RouterGroup, h *handler.NginxHandler, tokens *pluginsvc.IntegrationTokenService) {
	api.GET("/nginx/default", middleware.RequireScope("nginx:read"), gin.WrapF(h.GetDefault))
	api.PUT("/nginx/default", middleware.RequireScope("nginx:manage"), gin.WrapF(h.UpdateDefault))
	api.GET("/nginx/global", middleware.RequireScope("nginx:read"), gin.WrapF(h.GetGlobal))
	api.PUT("/nginx/global", middleware.RequireScope("nginx:manage"), gin.WrapF(h.UpdateGlobal))
	api.POST("/nginx/test", middleware.RequireScope("nginx:read"), gin.WrapF(h.Test))
	api.POST("/nginx/reload", middleware.RequireScope("nginx:manage"), gin.WrapF(h.Reload))
	_ = tokens
}

func registerSSLRoutes(api *gin.RouterGroup, h *handler.SSLHandler) {
	api.GET("/websites/:id/ssl", middleware.RequireScope("ssl:read"), gin.WrapF(h.GetStatus))
	api.PUT("/websites/:id/ssl/manual", middleware.RequireScope("ssl:write"), gin.WrapF(h.UpdateManual))
	api.POST("/websites/:id/ssl/certbot", middleware.RequireScope("ssl:write"), gin.WrapF(h.StartCertbot))
	api.GET("/websites/:id/ssl/certbot/stream", middleware.RequireScope("ssl:read"), gin.WrapF(h.CertbotStream))
}

func registerDockerRoutes(api *gin.RouterGroup, h *handler.DockerHandler, tokens *pluginsvc.IntegrationTokenService) {
	api.GET("/docker/containers", middleware.RequireScope("docker:read"), gin.WrapF(h.List))
	api.POST("/docker/containers/:id/restart", middleware.RequireScope("docker:manage"), gin.WrapF(h.Restart))
	api.POST("/docker/containers/:id/stop", middleware.RequireScope("docker:manage"), gin.WrapF(h.Stop))
	api.GET("/docker/containers/:id/logs", middleware.RequireScope("docker:read"), gin.WrapF(h.Logs))
	_ = tokens
}

func registerFilesRoutes(api *gin.RouterGroup, h *handler.FilesHandler) {
	api.GET("/files", middleware.RequireScope("files:read"), gin.WrapF(h.Browse))
	api.GET("/files/content", middleware.RequireScope("files:read"), gin.WrapF(h.Read))
	api.GET("/files/raw", middleware.RequireScope("files:read"), gin.WrapF(h.Raw))
	api.PUT("/files/content", middleware.RequireScope("files:manage"), gin.WrapF(h.Save))
	api.POST("/files", middleware.RequireScope("files:manage"), gin.WrapF(h.Create))
	api.POST("/files/actions", middleware.RequireScope("files:manage"), gin.WrapF(h.Action))
	api.POST("/files/batch-save", middleware.RequireScope("files:manage"), gin.WrapF(h.BatchSave))
	api.POST("/files/batch-delete", middleware.RequireScope("files:manage"), gin.WrapF(h.BatchDelete))
	api.DELETE("/files", middleware.RequireScope("files:manage"), gin.WrapF(h.Delete))
}

func registerMountRoutes(api *gin.RouterGroup, h *handler.MountHandler) {
	api.GET("/mounts", middleware.RequireScope("mount:read"), gin.WrapF(h.List))
	api.POST("/mounts", middleware.RequireScope("mount:manage"), gin.WrapF(h.Create))
	api.PUT("/mounts", middleware.RequireScope("mount:manage"), gin.WrapF(h.Update))
	api.DELETE("/mounts", middleware.RequireScope("mount:manage"), gin.WrapF(h.Delete))
	api.POST("/mounts/enable", middleware.RequireScope("mount:manage"), gin.WrapF(h.Enable))
}

func registerCronRoutes(api *gin.RouterGroup, h *handler.CronHandler) {
	api.GET("/cronjobs", middleware.RequireScope("cron:read"), gin.WrapF(h.List))
	api.POST("/cronjobs", middleware.RequireScope("cron:write"), gin.WrapF(h.Create))
	api.PUT("/cronjobs/:id", middleware.RequireScope("cron:write"), gin.WrapF(h.Update))
	api.DELETE("/cronjobs/:id", middleware.RequireScope("cron:write"), gin.WrapF(h.Delete))
	api.POST("/cronjobs/:id/run", middleware.RequireScope("cron:manage"), gin.WrapF(h.Run))
	api.GET("/cronjobs/:id/run/stream", middleware.RequireScope("cron:read"), gin.WrapF(h.RunStream))
}

func registerPluginPublicRoutes(api *gin.RouterGroup, h *handler.PluginHandler) {
	api.GET("/plugins", middleware.RequireScope("plugins:read"), gin.WrapF(h.List))
}

func registerPluginRoutes(api *gin.RouterGroup, h *handler.PluginHandler, configH *handler.ConfigHandler, keyH *handler.KeyringHandler, tokenH *handler.IntegrationTokenHandler) {
	api.GET("/plugins/catalog", gin.WrapF(h.CatalogList))
	api.GET("/plugins/catalog/:vendor/:name", gin.WrapF(h.CatalogGet))
	api.GET("/plugins/install/settings", gin.WrapF(h.InstallSettings))
	api.POST("/plugins/install/resolve", gin.WrapF(h.Resolve))
	api.POST("/plugins/install", gin.WrapF(h.Install))
	api.POST("/plugins/:vendor/:name/enable", gin.WrapF(h.Enable))
	api.POST("/plugins/:vendor/:name/disable", gin.WrapF(h.Disable))
	api.POST("/plugins/:vendor/:name/switch", gin.WrapF(h.Switch))
	api.POST("/plugins/:vendor/:name/restore-bundled", gin.WrapF(h.RestoreBundled))
	api.DELETE("/plugins/:vendor/:name/versions/:version", gin.WrapF(h.Uninstall))

	api.GET("/plugins/:vendor/:name/versions/:version/config", gin.WrapF(configH.Get))
	api.PUT("/plugins/:vendor/:name/versions/:version/config", gin.WrapF(configH.Put))

	api.GET("/plugins/keyring", gin.WrapF(keyH.List))
	api.POST("/plugins/keyring", gin.WrapF(keyH.Add))
	api.DELETE("/plugins/keyring", gin.WrapF(keyH.Revoke))

	api.GET("/plugins/permissions/registry", gin.WrapF(tokenH.Registry))
	api.GET("/plugins/:vendor/:name/integration-tokens", gin.WrapF(tokenH.List))
	api.POST("/plugins/:vendor/:name/integration-tokens", gin.WrapF(tokenH.Create))
	api.PATCH("/plugins/:vendor/:name/integration-tokens/:tokenId", gin.WrapF(tokenH.Patch))
	api.DELETE("/plugins/:vendor/:name/integration-tokens/:tokenId", gin.WrapF(tokenH.Delete))
}

func registerObservabilityRoutes(api *gin.RouterGroup, h *handler.ObservabilityHandler) {
	api.GET("/query/meta", middleware.RequireScope("query:read"), gin.WrapF(h.QueryMeta))
	api.GET("/query", middleware.RequireScope("query:read"), gin.WrapF(h.QueryGet))
	api.POST("/query", middleware.RequireScope("query:read"), gin.WrapF(h.Query))
	api.GET("/query/tail", middleware.RequireScope("query:read"), gin.WrapF(h.Tail))
	api.GET("/query/saved", middleware.RequireScope("query:read"), gin.WrapF(h.ListSavedQueries))
	api.POST("/query/saved", middleware.RequireScope("query:manage"), gin.WrapF(h.SaveQuery))
	api.PATCH("/query/saved/:id", middleware.RequireScope("query:manage"), gin.WrapF(h.UpdateSavedQuery))
	api.DELETE("/query/saved/:id", middleware.RequireScope("query:manage"), gin.WrapF(h.DeleteSavedQuery))
	api.GET("/metrics/traffic/series", middleware.RequireScope("metrics:read"), gin.WrapF(h.TrafficSeries))
	api.GET("/metrics/traffic/top-sites", middleware.RequireScope("metrics:read"), gin.WrapF(h.TrafficTopSites))
	api.GET("/metrics/traffic/status-codes", middleware.RequireScope("metrics:read"), gin.WrapF(h.TrafficStatusCodes))
	api.GET("/metrics/traffic/summary", middleware.RequireScope("metrics:read"), gin.WrapF(h.TrafficSummary))
	api.GET("/metrics/nginx/current", middleware.RequireScope("metrics:read"), gin.WrapF(h.NginxCurrent))
	api.GET("/metrics/nginx/series", middleware.RequireScope("metrics:read"), gin.WrapF(h.NginxSeries))
	api.GET("/metrics/nginx/vts/status", middleware.RequireScope("metrics:read"), gin.WrapF(h.NginxVTSStatus))
	api.GET("/metrics/nginx/vts/servers", middleware.RequireScope("metrics:read"), gin.WrapF(h.NginxVTSServers))
	api.GET("/metrics/nginx/vts/upstreams", middleware.RequireScope("metrics:read"), gin.WrapF(h.NginxVTSUpstreams))
}

func registerTerminalRoutes(api *gin.RouterGroup, h *handler.TerminalHandler) {
	api.GET("/terminal/ws", h.HandleWS)
	api.GET("/terminal/sessions", h.ListSessions)
	api.GET("/terminal/sessions/:id/snapshot", h.GetSnapshot)
	api.DELETE("/terminal/sessions/:id", h.KillSession)
}
