package server

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/jahrulnr/gosite/internal/config"
)

// HTTPS serves handler over TLS using configured certificate paths.
func HTTPS(cfg config.Config, handler http.Handler) error {
	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	if err := server.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve tls: %w", err)
	}
	return nil
}
