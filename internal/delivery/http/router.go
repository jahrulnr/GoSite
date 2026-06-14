package http

import (
	"context"
	"database/sql"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/delivery/http/handler"
	"github.com/jahrulnr/gosite/internal/delivery/http/frontend"
	"github.com/jahrulnr/gosite/internal/delivery/http/middleware"
	"github.com/jahrulnr/gosite/internal/infra/commander"
	dockerinfra "github.com/jahrulnr/gosite/internal/infra/docker"
	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/observability/grafanalite"
	"github.com/jahrulnr/gosite/internal/observability/splunklite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	cronsvc "github.com/jahrulnr/gosite/internal/service/cron"
	"github.com/jahrulnr/gosite/internal/service/database"
	dockersvc "github.com/jahrulnr/gosite/internal/service/docker"
	filessvc "github.com/jahrulnr/gosite/internal/service/files"
	"github.com/jahrulnr/gosite/internal/service/logs"
	mountsvc "github.com/jahrulnr/gosite/internal/service/mount"
	"github.com/jahrulnr/gosite/internal/service/settings"
	"github.com/jahrulnr/gosite/internal/service/ssl"
	"github.com/jahrulnr/gosite/internal/service/system"
	"github.com/jahrulnr/gosite/internal/service/uimeta"
	"github.com/jahrulnr/gosite/internal/service/website"
)

// NewRouter wires the Gin engine with API routes.
func NewRouter(cfg config.Config, db *sql.DB) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	sessions := auth.NewStoreWithOptions(0, cfg.SessionCookieSecure)
	users := sqlite.NewUserRepository(db)
	lockscreen := auth.NewLockscreen()
	authSvc := auth.NewService(users, sessions, auth.WithLockscreen(lockscreen))

	healthHandler := handler.NewHealthHandler()
	authHandler := handler.NewAuthHandler(authSvc, sessions, auth.LoginMetadataFromConfig(
		cfg.EnableLockscreen,
		cfg.AuthEnable,
		int(cfg.LockAfter.Seconds()),
		cfg.WebPath,
		cfg.Storage,
	))

	storage := cfg.Storage
	webconfig := filepath.Join(storage, "webconfig")
	nginxEtc := "/etc/nginx"
	paths := nginx.Paths{
		Storage:       storage,
		SiteD:         filepath.Join(webconfig, "site.d"),
		ActiveD:       filepath.Join(webconfig, "active.d"),
		Backups:       filepath.Join(webconfig, "backups"),
		StaticTpl:     filepath.Join(webconfig, "site.conf"),
		ProxyTpl:      filepath.Join(webconfig, "site-proxy.conf"),
		NginxConf:     filepath.Join(webconfig, "nginx.conf"),
		GlobalConf:    filepath.Join(nginxEtc, "nginx.conf"),
		DefaultConf:   filepath.Join(nginxEtc, "http.d/default.conf"),
		SSLDefaultDir: filepath.Join(webconfig, "ssl/live/default"),
	}
	cmd := commander.NewExecRunner()
	baseRunner := nginx.NewRunner(cmd, nginx.RunnerConfig{
		SiteDDir:   paths.SiteD,
		BackupsDir: paths.Backups,
		NginxConf:  paths.NginxConf,
	})
	var runner contracts.NginxRunner = baseRunner
	if cfg.AppEnv == "local" {
		runner = nginx.NewNoopReloadRunner(baseRunner)
	}
	ngx := nginx.NewService(runner, cmd, paths)
	websiteRepo := sqlite.NewWebsiteRepository(db)
	jobRepo := sqlite.NewJobRepository(db)
	websiteSvc := website.NewService(websiteRepo, ngx, cfg.WebPath)
	sslSvc := ssl.NewService(websiteRepo, jobRepo, ngx)

	websiteHandler := handler.NewWebsiteHandler(websiteSvc)
	nginxHandler := handler.NewNginxHandler(ngx)
	sslHandler := handler.NewSSLHandler(sslSvc)

	auditRepo := sqlite.NewAuditRepository(db)
	logRepo := sqlite.NewLogEventRepository(db)
	metricsRepo := sqlite.NewTrafficMetricsRepository(db)
	savedRepo := sqlite.NewSavedQueryRepository(db)
	splunkSvc := splunklite.NewService(auditRepo, jobRepo, logRepo, savedRepo, cfg.AuditRetentionDays, cfg.LogEventsRetentionDays)
	grafanaSvc := grafanalite.NewService(metricsRepo)
	logDir := filepath.Join(storage, "laravel", "logs")
	logsSvc := logs.NewService(logDir, websiteRepo)
	queryMetaSvc := splunklite.NewMetaService(logsSvc, logDir)
	logIngestor := splunklite.NewLogIngestor(logRepo, logDir)
	splunkSvc.SetIngestor(logIngestor)
	obsHandler := handler.NewObservabilityHandler(splunkSvc, queryMetaSvc, logIngestor, grafanaSvc)

	systemSvc := system.NewService(logDir, nil, system.CommandAdapter{Runner: cmd})
	settingsSvc := settings.NewService(users)
	databaseSvc := database.NewService(db, cfg.Database)
	uimetaSvc := uimeta.NewService(cfg)

	systemHandler := handler.NewSystemHandler(systemSvc)
	settingsHandler := handler.NewSettingsHandler(settingsSvc, authSvc)
	logsHandler := handler.NewLogsHandler(logsSvc)
	databaseHandler := handler.NewDatabaseHandler(databaseSvc)
	uimetaHandler := handler.NewUIMetaHandler(uimetaSvc)
	dashboardHandler := handler.NewDashboardHandler(systemSvc, sslSvc, splunkSvc, grafanaSvc)

	dockerClient, err := dockerinfra.NewClient("")
	var dockerSvc contracts.DockerClient
	if err != nil {
		dockerSvc = dockerinfra.NoopClient{}
	} else {
		dockerSvc = dockerClient
	}
	dockerService := dockersvc.NewService(dockerSvc)
	dockerHandler := handler.NewDockerHandler(dockerService)

	fileRoots := []string{cfg.WebPath, cfg.Storage, "/tmp"}
	filesSvc := filessvc.NewService(fileRoots, cfg.FilesAllowExecute, cmd)
	filesHandler := handler.NewFilesHandler(filesSvc)

	fstabPath := filepath.Join(cfg.EtcDir, "fstab")
	mountSvc := mountsvc.NewService(fstabPath, cmd)
	mountHandler := handler.NewMountHandler(mountSvc)

	worker := job.NewWorker(jobRepo, cmd, 32)
	worker.Start(context.Background(), 2)

	cronRepo := sqlite.NewCronJobRepository(db)
	cronSvc := cronsvc.NewService(cronRepo, jobRepo, worker)
	cronHandler := handler.NewCronHandler(cronSvc, worker)

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
	protected.Use(middleware.RequireSession(authSvc))
	registerWebsiteRoutes(protected, websiteHandler)
	registerNginxRoutes(protected, nginxHandler)
	registerSSLRoutes(protected, sslHandler)
	registerDockerRoutes(protected, dockerHandler)
	registerFilesRoutes(protected, filesHandler)
	registerMountRoutes(protected, mountHandler)
	registerCronRoutes(protected, cronHandler)
	registerObservabilityRoutes(protected, obsHandler)
	registerDashboardRoutes(protected, dashboardHandler)
	registerSystemRoutes(protected, systemHandler)
	registerSettingsRoutes(protected, settingsHandler)
	registerLogsRoutes(protected, logsHandler)
	registerDatabaseRoutes(protected, databaseHandler)
	registerUIMetaRoutes(protected, uimetaHandler)

	if cfg.FEEmbed {
		frontend.Register(engine, frontend.DistFS)
	}

	return engine
}

func registerDashboardRoutes(api *gin.RouterGroup, h *handler.DashboardHandler) {
	api.GET("/dashboard", gin.WrapF(h.Get))
}

func registerSystemRoutes(api *gin.RouterGroup, h *handler.SystemHandler) {
	api.GET("/system/info", gin.WrapF(h.Info))
	api.GET("/system/network", gin.WrapF(h.Network))
	api.GET("/system/disk-io", gin.WrapF(h.DiskIO))
	api.GET("/system/nginx-traffic", gin.WrapF(h.NginxTraffic))
}

func registerSettingsRoutes(api *gin.RouterGroup, h *handler.SettingsHandler) {
	api.PUT("/settings/profile", gin.WrapF(h.UpdateProfile))
}

func registerLogsRoutes(api *gin.RouterGroup, h *handler.LogsHandler) {
	api.GET("/logs/sites", gin.WrapF(h.ListSites))
	api.GET("/logs", gin.WrapF(h.Tail))
}

func registerDatabaseRoutes(api *gin.RouterGroup, h *handler.DatabaseHandler) {
	api.GET("/database/tables", gin.WrapF(h.ListTables))
	api.GET("/database/tables/:name", gin.WrapF(h.GetTable))
}

func registerUIMetaRoutes(api *gin.RouterGroup, h *handler.UIMetaHandler) {
	api.GET("/ui/meta", gin.WrapF(h.Get))
}

func registerWebsiteRoutes(api *gin.RouterGroup, h *handler.WebsiteHandler) {
	api.GET("/websites", gin.WrapF(h.List))
	api.POST("/websites", gin.WrapF(h.Create))
	api.POST("/websites/validate", gin.WrapF(h.Validate))
	api.GET("/websites/:id", gin.WrapF(h.Get))
	api.PUT("/websites/:id", gin.WrapF(h.Update))
	api.DELETE("/websites/:id", gin.WrapF(h.Delete))
	api.PATCH("/websites/:id/toggle", gin.WrapF(h.Toggle))
	api.GET("/websites/:id/nginx-config", gin.WrapF(h.GetNginxConfig))
	api.PUT("/websites/:id/nginx-config", gin.WrapF(h.UpdateNginxConfig))
}

func registerNginxRoutes(api *gin.RouterGroup, h *handler.NginxHandler) {
	api.GET("/nginx/default", gin.WrapF(h.GetDefault))
	api.PUT("/nginx/default", gin.WrapF(h.UpdateDefault))
	api.GET("/nginx/global", gin.WrapF(h.GetGlobal))
	api.PUT("/nginx/global", gin.WrapF(h.UpdateGlobal))
	api.POST("/nginx/test", gin.WrapF(h.Test))
	api.POST("/nginx/reload", gin.WrapF(h.Reload))
}

func registerSSLRoutes(api *gin.RouterGroup, h *handler.SSLHandler) {
	api.GET("/websites/:id/ssl", gin.WrapF(h.GetStatus))
	api.PUT("/websites/:id/ssl/manual", gin.WrapF(h.UpdateManual))
	api.POST("/websites/:id/ssl/certbot", gin.WrapF(h.StartCertbot))
	api.GET("/websites/:id/ssl/certbot/stream", gin.WrapF(h.CertbotStream))
}

func registerDockerRoutes(api *gin.RouterGroup, h *handler.DockerHandler) {
	api.GET("/docker/containers", gin.WrapF(h.List))
	api.POST("/docker/containers/:id/restart", gin.WrapF(h.Restart))
	api.POST("/docker/containers/:id/stop", gin.WrapF(h.Stop))
	api.GET("/docker/containers/:id/logs", gin.WrapF(h.Logs))
}

func registerFilesRoutes(api *gin.RouterGroup, h *handler.FilesHandler) {
	api.GET("/files", gin.WrapF(h.Browse))
	api.GET("/files/content", gin.WrapF(h.Read))
	api.POST("/files", gin.WrapF(h.Create))
	api.POST("/files/actions", gin.WrapF(h.Action))
	api.DELETE("/files", gin.WrapF(h.Delete))
}

func registerMountRoutes(api *gin.RouterGroup, h *handler.MountHandler) {
	api.GET("/mounts", gin.WrapF(h.List))
	api.POST("/mounts", gin.WrapF(h.Create))
	api.PUT("/mounts", gin.WrapF(h.Update))
	api.DELETE("/mounts", gin.WrapF(h.Delete))
	api.POST("/mounts/enable", gin.WrapF(h.Enable))
}

func registerCronRoutes(api *gin.RouterGroup, h *handler.CronHandler) {
	api.GET("/cronjobs", gin.WrapF(h.List))
	api.POST("/cronjobs", gin.WrapF(h.Create))
	api.PUT("/cronjobs/:id", gin.WrapF(h.Update))
	api.DELETE("/cronjobs/:id", gin.WrapF(h.Delete))
	api.POST("/cronjobs/:id/run", gin.WrapF(h.Run))
	api.GET("/cronjobs/:id/run/stream", gin.WrapF(h.RunStream))
}

func registerObservabilityRoutes(api *gin.RouterGroup, h *handler.ObservabilityHandler) {
	api.GET("/query/meta", gin.WrapF(h.QueryMeta))
	api.POST("/query", gin.WrapF(h.Query))
	api.GET("/query/tail", gin.WrapF(h.Tail))
	api.GET("/query/saved", gin.WrapF(h.ListSavedQueries))
	api.POST("/query/saved", gin.WrapF(h.SaveQuery))
	api.PATCH("/query/saved/:id", gin.WrapF(h.UpdateSavedQuery))
	api.DELETE("/query/saved/:id", gin.WrapF(h.DeleteSavedQuery))
	api.GET("/metrics/traffic/series", gin.WrapF(h.TrafficSeries))
	api.GET("/metrics/traffic/top-sites", gin.WrapF(h.TrafficTopSites))
	api.GET("/metrics/traffic/status-codes", gin.WrapF(h.TrafficStatusCodes))
	api.GET("/metrics/traffic/summary", gin.WrapF(h.TrafficSummary))
}
