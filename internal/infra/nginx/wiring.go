package nginx

import (
	"path/filepath"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/infra/commander"
)

// PathsFromConfig builds nginx filesystem paths from panel config.
func PathsFromConfig(cfg config.Config) Paths {
	storage := cfg.Storage
	webconfig := filepath.Join(storage, "webconfig")
	nginxEtc := "/etc/nginx"
	if cfg.EtcDir != "" {
		nginxEtc = filepath.Join(cfg.EtcDir, "nginx")
	}
	return Paths{
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
}

// NewServiceFromConfig wires a production or local nginx service.
func NewServiceFromConfig(cfg config.Config, cmd contracts.CommandRunner) *Service {
	if cmd == nil {
		cmd = commander.NewExecRunner()
	}
	paths := PathsFromConfig(cfg)
	baseRunner := NewRunner(cmd, RunnerConfig{
		SiteDDir:   paths.SiteD,
		BackupsDir: paths.Backups,
		NginxConf:  paths.GlobalConf,
	})
	var runner contracts.NginxRunner = baseRunner
	if cfg.AppEnv == "local" {
		runner = NewNoopReloadRunner(baseRunner)
	}
	return NewService(runner, cmd, paths)
}
