package app

import (
	"context"
	"log"
	"time"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/infra/nginx"
)

const nginxWatchInterval = 30 * time.Second

func runNginxWatchdog(ctx context.Context, cfg config.Config) {
	if cfg.AppEnv == "local" {
		return
	}
	svc := nginx.NewServiceFromConfig(cfg, nil)
	ticker := time.NewTicker(nginxWatchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := svc.EnsureRunning(ctx); err != nil {
				log.Printf("nginx watchdog: %v", err)
			}
		}
	}
}
