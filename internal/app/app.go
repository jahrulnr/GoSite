package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jahrulnr/gosite/internal/config"
	delivery "github.com/jahrulnr/gosite/internal/delivery/http"
	"github.com/jahrulnr/gosite/internal/infra/commander"
	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/observability/grafanalite"
	"github.com/jahrulnr/gosite/internal/observability/nginxlite"
	"github.com/jahrulnr/gosite/internal/observability/splunklite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/server"
)

// RunServe starts the HTTPS API and background workers.
func RunServe(cfg config.Config) error {
	db, err := sqlite.Open(cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := sqlite.Migrate(db, cfg.MigrationsDir); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	engine := delivery.NewRouter(cfg, db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logDir := cfg.LogsDir()
	metricsRepo := sqlite.NewTrafficMetricsRepository(db)
	offsetPath := filepath.Join(cfg.Storage, "gosite", "metrics_offsets.json")
	collector := grafanalite.NewCollector(logDir, offsetPath, metricsRepo, cfg.LogEventsRetentionDays)
	nginxStatusRepo := sqlite.NewNginxStatusRepository(db)
	nginxVTSRepo := sqlite.NewNginxVTSRepository(db)
	nginxCollector := nginxlite.NewCollector(cfg.NginxStubStatusURL, nginxStatusRepo, cfg.LogEventsRetentionDays)
	vtsCollector := nginxlite.NewVTSCollector(cfg.NginxVTSStatusURL, nginxVTSRepo, cfg.LogEventsRetentionDays)

	auditRepo := sqlite.NewAuditRepository(db)
	logRepo := sqlite.NewLogEventRepository(db)
	splunkSvc := splunklite.NewService(auditRepo, sqlite.NewJobRepository(db), logRepo, sqlite.NewSavedQueryRepository(db), cfg.AuditRetentionDays, cfg.LogEventsRetentionDays)

	cmd := commander.NewExecRunner()
	jobRepo := sqlite.NewJobRepository(db)
	worker := job.NewWorker(jobRepo, cmd, 32)
	worker.Start(ctx, 2)

	cronRepo := sqlite.NewCronJobRepository(db)
	go runCronScheduler(ctx, cronRepo, jobRepo, worker.Enqueue)

	go runMetricsCollector(ctx, collector)
	go runNginxStatusCollector(ctx, nginxCollector)
	go runNginxVTSCollector(ctx, vtsCollector)
	go runRetentionPurge(ctx, db, splunkSvc, collector, nginxCollector, vtsCollector)
	go runNginxWatchdog(ctx, cfg)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	if cfg.TLSEnable {
		log.Printf("gosite serve listening on %s (tls, panel)", cfg.ListenAddr)
		return server.HTTPS(cfg, engine)
	}
	log.Printf("gosite serve listening on %s (http)", cfg.ListenAddr)
	return server.HTTP(cfg, engine)
}

func runMetricsCollector(ctx context.Context, collector *grafanalite.Collector) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	if err := collector.Collect(ctx); err != nil {
		log.Printf("metrics collector: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := collector.Collect(ctx); err != nil {
				log.Printf("metrics collector: %v", err)
			}
		}
	}
}

func runNginxStatusCollector(ctx context.Context, collector *nginxlite.Collector) {
	if collector == nil {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	if err := collector.Collect(ctx); err != nil {
		log.Printf("nginx status collector: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := collector.Collect(ctx); err != nil {
				log.Printf("nginx status collector: %v", err)
			}
		}
	}
}

func runNginxVTSCollector(ctx context.Context, collector *nginxlite.VTSCollector) {
	if collector == nil {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	if err := collector.Collect(ctx); err != nil {
		log.Printf("nginx vts collector: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := collector.Collect(ctx); err != nil {
				log.Printf("nginx vts collector: %v", err)
			}
		}
	}
}

func runRetentionPurge(ctx context.Context, db *sql.DB, splunk *splunklite.Service, collector *grafanalite.Collector, nginxCollector *nginxlite.Collector, vtsCollector *nginxlite.VTSCollector) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UTC()
			if err := splunk.PurgeRetention(ctx, now); err != nil {
				log.Printf("splunk retention: %v", err)
			}
			if err := collector.PurgeRetention(ctx); err != nil {
				log.Printf("grafana retention: %v", err)
			}
			if nginxCollector != nil {
				if err := nginxCollector.PurgeRetention(ctx); err != nil {
					log.Printf("nginx status retention: %v", err)
				}
			}
			if vtsCollector != nil {
				if err := vtsCollector.PurgeRetention(ctx); err != nil {
					log.Printf("nginx vts retention: %v", err)
				}
			}
			if err := sqlite.Vacuum(db); err != nil {
				log.Printf("sqlite vacuum: %v", err)
			}
		}
	}
}
