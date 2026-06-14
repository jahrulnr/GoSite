package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jahrulnr/gosite/internal/config"
)

// HTTP serves handler over plain HTTP (for use behind a TLS-terminating reverse proxy).
func HTTP(cfg config.Config, handler http.Handler) error {
	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve: %w", err)
	}
	return nil
}
