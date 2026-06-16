package ssl

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Service manages SSL certificates and certbot jobs.
type Service struct {
	websites *sqlite.WebsiteRepository
	jobs     *sqlite.JobRepository
	nginx    *nginx.Service
	worker   *job.Worker
	hooks    contracts.HookBus
}

// Option configures SSL service dependencies.
type Option func(*Service)

// WithHookBus dispatches SSL lifecycle events to plugins.
func WithHookBus(hooks contracts.HookBus) Option {
	return func(s *Service) {
		if hooks != nil {
			s.hooks = hooks
		}
	}
}

// NewService returns an SSL service.
func NewService(websites *sqlite.WebsiteRepository, jobs *sqlite.JobRepository, ngx *nginx.Service, worker *job.Worker, opts ...Option) *Service {
	svc := &Service{websites: websites, jobs: jobs, nginx: ngx, worker: worker, hooks: contracts.NoopHookBus{}}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// Status describes SSL state for a website.
type Status struct {
	Enabled    bool       `json:"enabled"`
	CertPath   string     `json:"cert_path,omitempty"`
	KeyPath    string     `json:"key_path,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	Expired    bool       `json:"expired"`
	PublicPEM  string     `json:"public_pem,omitempty"`
	PrivatePEM string     `json:"private_pem,omitempty"`
}

// ManualInput holds PEM content for manual SSL upload.
type ManualInput struct {
	Public  string
	Private string
}

// EnqueueCertbot creates a pending certbot job for the domain.
func (s *Service) EnqueueCertbot(ctx context.Context, websiteID int64) (sqlite.JobRun, error) {
	site, err := s.websites.FindByID(ctx, websiteID)
	if err != nil {
		return sqlite.JobRun{}, apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}

	if err := s.prepareForCertbot(ctx, site); err != nil {
		return sqlite.JobRun{}, err
	}
	if _, err := s.hooks.Dispatch(ctx, "ssl.before_issue", map[string]any{
		"website_id": site.ID,
		"domain":     site.Domain,
	}); err != nil {
		return sqlite.JobRun{}, err
	}

	cmd := fmt.Sprintf(
		"certbot --non-interactive --agree-tos --register-unsafely-without-email --nginx -d %s",
		site.Domain,
	)
	job, err := s.jobs.Create(ctx, sqlite.JobRun{
		JobType: "certbot",
		Name:    site.Domain,
		Status:  sqlite.JobStatusPending,
		Output:  cmd,
	})
	if err != nil {
		return sqlite.JobRun{}, apperror.Wrap(apperror.CodeDatabase, "enqueue certbot job", err)
	}
	if s.worker != nil {
		s.worker.Enqueue(job.ID)
	}
	return job, nil
}

// GetCertbotJob returns a certbot job by id.
func (s *Service) GetCertbotJob(ctx context.Context, jobID int64) (sqlite.JobRun, error) {
	job, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return sqlite.JobRun{}, apperror.Wrap(apperror.CodeNotFound, "job not found", err)
	}
	return job, nil
}

// UpdateManual uploads PEM files and updates site.d ssl directives.
func (s *Service) UpdateManual(ctx context.Context, websiteID int64, in ManualInput) error {
	site, err := s.websites.FindByID(ctx, websiteID)
	if err != nil {
		return apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}

	public := strings.ReplaceAll(in.Public, "\r", "")
	private := strings.ReplaceAll(in.Private, "\r", "")
	if err := validatePEM(public, private); err != nil {
		return err
	}

	paths := s.nginx.Paths()
	archiveDir := filepath.Join(paths.Storage, "webconfig/ssl/archive", site.Domain)
	liveDir := filepath.Join(paths.Storage, "webconfig/ssl/live", site.Domain)

	config, readErr := s.nginx.ReadSiteConfig(ctx, site.Domain)
	var publicPath, privatePath string

	if readErr != nil {
		if err := os.MkdirAll(archiveDir, 0o755); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "create archive dir", err)
		}
		publicPath = filepath.Join(archiveDir, "fullchain.pem")
		privatePath = filepath.Join(archiveDir, "privkey.pem")
		if err := os.WriteFile(publicPath, []byte(public), 0o600); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write public cert", err)
		}
		if err := os.WriteFile(privatePath, []byte(private), 0o600); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write private key", err)
		}
		if err := os.MkdirAll(liveDir, 0o755); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "create live dir", err)
		}
		liveCert := filepath.Join(liveDir, "cert.pem")
		liveKey := filepath.Join(liveDir, "key.pem")
		if err := os.WriteFile(liveCert, []byte(public), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(liveKey, []byte(private), 0o600); err != nil {
			return err
		}
		publicPath = liveCert
		privatePath = liveKey
	} else {
		existingCert, existingKey, ok := nginx.ParseCertPaths(config)
		if ok {
			publicPath = existingCert
			privatePath = existingKey
		} else {
			if err := os.MkdirAll(liveDir, 0o755); err != nil {
				return err
			}
			publicPath = filepath.Join(liveDir, "cert.pem")
			privatePath = filepath.Join(liveDir, "key.pem")
		}
		if err := os.WriteFile(publicPath, []byte(public), 0o644); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write public cert", err)
		}
		if err := os.WriteFile(privatePath, []byte(private), 0o600); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "write private key", err)
		}
	}

	updated := config
	if readErr != nil {
		updated = ""
	}
	updated = nginx.UpdateSSLDirectives(updated, publicPath, privatePath)

	if readErr != nil {
		if err := s.nginx.WriteSiteConfig(ctx, site.Domain, updated); err != nil {
			return err
		}
	} else {
		if err := s.nginx.UpdateSiteConfig(ctx, site.Domain, updated); err != nil {
			return err
		}
	}
	if err := s.nginx.Reload(ctx); err != nil {
		return err
	}

	site.SSL = true
	_, err = s.websites.Update(ctx, site)
	if err != nil {
		return apperror.Wrap(apperror.CodeDatabase, "update ssl flag", err)
	}
	_, _ = s.hooks.Dispatch(ctx, "ssl.after_renew", map[string]any{
		"website_id": site.ID,
		"domain":     site.Domain,
		"manual":     true,
	})
	return nil
}

// GetStatus returns SSL status and PEM content when readable.
func (s *Service) GetStatus(ctx context.Context, websiteID int64) (Status, error) {
	site, err := s.websites.FindByID(ctx, websiteID)
	if err != nil {
		return Status{}, apperror.Wrap(apperror.CodeNotFound, "website not found", err)
	}

	config, err := s.nginx.ReadSiteConfig(ctx, site.Domain)
	if err != nil {
		return Status{Enabled: false}, nil
	}

	certPath, keyPath, ok := nginx.ParseCertPaths(config)
	if !ok {
		return Status{Enabled: false}, nil
	}

	status := Status{
		Enabled:  true,
		CertPath: certPath,
		KeyPath:  keyPath,
	}

	if data, err := os.ReadFile(certPath); err == nil {
		status.PublicPEM = string(data)
		if exp, expired, parseErr := ParseCertExpiry(data); parseErr == nil {
			status.ExpiresAt = &exp
			status.Expired = expired
		}
	}
	if data, err := os.ReadFile(keyPath); err == nil {
		status.PrivatePEM = string(data)
	}

	return status, nil
}

// ExpiringCert describes a site certificate nearing expiry.
type ExpiringCert struct {
	WebsiteID int64     `json:"website_id"`
	Domain    string    `json:"domain"`
	ExpiresAt time.Time `json:"expires_at"`
	DaysLeft  int       `json:"days_left"`
	Expired   bool      `json:"expired"`
}

// ListExpiring returns enabled SSL sites expiring within withinDays.
func (s *Service) ListExpiring(ctx context.Context, withinDays int) ([]ExpiringCert, error) {
	if withinDays <= 0 {
		withinDays = 30
	}
	sites, err := s.websites.List(ctx)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "list websites failed", err)
	}

	deadline := time.Now().AddDate(0, 0, withinDays)
	var out []ExpiringCert
	for _, site := range sites {
		status, err := s.GetStatus(ctx, site.ID)
		if err != nil || !status.Enabled || status.ExpiresAt == nil {
			continue
		}
		exp := *status.ExpiresAt
		if exp.After(deadline) && !status.Expired {
			continue
		}
		daysLeft := int(time.Until(exp).Hours() / 24)
		out = append(out, ExpiringCert{
			WebsiteID: site.ID,
			Domain:    site.Domain,
			ExpiresAt: exp,
			DaysLeft:  daysLeft,
			Expired:   status.Expired,
		})
	}
	return out, nil
}

// ParseCertExpiry parses PEM and returns notAfter and whether cert is expired.
func ParseCertExpiry(pemData []byte) (time.Time, bool, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return time.Time{}, false, apperror.New(apperror.CodeSSLInvalid, "invalid pem certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, false, apperror.Wrap(apperror.CodeSSLInvalid, "parse certificate", err)
	}
	expired := time.Now().After(cert.NotAfter)
	return cert.NotAfter, expired, nil
}

func validatePEM(public, private string) error {
	if !strings.Contains(public, "BEGIN CERTIFICATE") {
		return apperror.New(apperror.CodeSSLInvalid, "invalid public certificate")
	}
	if !strings.Contains(private, "BEGIN") || !strings.Contains(private, "PRIVATE KEY") {
		return apperror.New(apperror.CodeSSLInvalid, "invalid private key")
	}
	return nil
}

// prepareForCertbot repoints placeholder domain certs to the default vhost material,
// reloads nginx so certbot's pre-patch nginx -t passes, then removes the placeholder
// live directory so certbot can create a Let's Encrypt lineage.
func (s *Service) prepareForCertbot(ctx context.Context, site sqlite.Website) error {
	paths := s.nginx.Paths()
	liveDir := filepath.Join(paths.Storage, "webconfig/ssl/live", site.Domain)
	if !isPlaceholderSSL(liveDir) {
		return nil
	}

	config, err := s.nginx.ReadSiteConfig(ctx, site.Domain)
	if err != nil {
		return apperror.Wrap(apperror.CodeNotFound, "site config not found", err)
	}

	certPath, keyPath, ok := nginx.ParseCertPaths(config)
	domainCert, domainKey := domainSSLPaths(paths.Storage, site.Domain)
	defaultCert := filepath.Join(paths.SSLDefaultDir, "cert.pem")
	defaultKey := filepath.Join(paths.SSLDefaultDir, "key.pem")

	if ok && sslPathsEqual(certPath, domainCert) && sslPathsEqual(keyPath, domainKey) {
		updated := nginx.UpdateSSLDirectives(config, defaultCert, defaultKey)
		if err := s.nginx.UpdateSiteConfig(ctx, site.Domain, updated); err != nil {
			return apperror.Wrap(apperror.CodeNginxTestFailed, "stage default ssl for certbot", err)
		}
		if err := s.nginx.Reload(ctx); err != nil {
			return apperror.Wrap(apperror.CodeNginxReloadFailed, "reload nginx for certbot prep", err)
		}
	}

	if err := clearPlaceholderSSL(paths.Storage, site.Domain); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "prepare certbot ssl storage", err)
	}
	return nil
}

func domainSSLPaths(storage, domain string) (cert, key string) {
	base := filepath.Join(storage, "webconfig/ssl/live", domain)
	return filepath.Join(base, "cert.pem"), filepath.Join(base, "key.pem")
}

func sslPathsEqual(a, b string) bool {
	if a == b {
		return true
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && absA == absB
}

// clearPlaceholderSSL removes gosite self-signed files from live/{domain} so certbot can
// create a Let's Encrypt lineage. /etc/letsencrypt symlinks to webconfig/ssl, so the
// placeholder written at website create blocks certbot with CertStorageError.
func clearPlaceholderSSL(storage, domain string) error {
	liveDir := filepath.Join(storage, "webconfig/ssl/live", domain)
	if !isPlaceholderSSL(liveDir) {
		return nil
	}
	return os.RemoveAll(liveDir)
}

func isPlaceholderSSL(liveDir string) bool {
	info, err := os.Stat(liveDir)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, err := os.Lstat(filepath.Join(liveDir, "fullchain.pem")); err == nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(liveDir, "cert.pem")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(liveDir, "key.pem")); err != nil {
		return false
	}
	return true
}
