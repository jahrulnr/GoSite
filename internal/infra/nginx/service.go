package nginx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Paths holds nginx-related filesystem locations.
type Paths struct {
	Storage       string
	SiteD         string
	ActiveD       string
	Backups       string
	StaticTpl     string
	ProxyTpl      string
	NginxConf     string
	GlobalConf    string
	DefaultConf   string
	SSLDefaultDir string
}

// Service coordinates nginx config files, symlinks, and reload.
type Service struct {
	runner contracts.NginxRunner
	cmd    contracts.CommandRunner
	paths  Paths
}

// NewService returns an nginx filesystem service.
func NewService(runner contracts.NginxRunner, cmd contracts.CommandRunner, paths Paths) *Service {
	return &Service{runner: runner, cmd: cmd, paths: paths}
}

// Paths returns configured filesystem locations.
func (s *Service) Paths() Paths {
	return s.paths
}

// TestConfig validates config content using an isolated nginx.conf clone.
// The site config is written to a temp file only — site.d is not modified.
func (s *Service) TestConfig(ctx context.Context, domain, content string) error {
	tmpSitePath := filepath.Join(os.TempDir(), fmt.Sprintf("nginx-site-test-%s-%d.conf", domain, time.Now().UnixNano()))
	if err := os.WriteFile(tmpSitePath, []byte(content), 0o644); err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "write temp site config", err)
	}
	defer os.Remove(tmpSitePath)

	baseConf, err := os.ReadFile(s.paths.NginxConf)
	if err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "read nginx.conf", err)
	}
	adjusted := replaceSiteIncludeForTest(string(baseConf), s.paths.SiteD, tmpSitePath)
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("nginx-test-%d.conf", time.Now().UnixNano()))
	if err := os.WriteFile(tmpPath, []byte(adjusted), 0o644); err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "write temp nginx conf", err)
	}
	defer os.Remove(tmpPath)

	if r, ok := s.runner.(*Runner); ok {
		return r.TestWithConfig(ctx, tmpPath)
	}
	return s.runner.Test(ctx)
}

// TestRawConfig validates a full nginx.conf (events/http at top level).
func (s *Service) TestRawConfig(ctx context.Context, content string) error {
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("nginx-raw-%d.conf", time.Now().UnixNano()))
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "write temp config", err)
	}
	defer os.Remove(tmpPath)

	if r, ok := s.runner.(*Runner); ok {
		return r.TestWithConfig(ctx, tmpPath)
	}
	return s.runner.Test(ctx)
}

// TestDefaultConfig validates http.d/default.conf (a server block) inside a clone of the production nginx.conf.
func (s *Service) TestDefaultConfig(ctx context.Context, content string) error {
	tmpDefaultPath := filepath.Join(os.TempDir(), fmt.Sprintf("nginx-default-test-%d.conf", time.Now().UnixNano()))
	if err := os.WriteFile(tmpDefaultPath, []byte(content), 0o644); err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "write temp default config", err)
	}
	defer os.Remove(tmpDefaultPath)

	baseConf, err := os.ReadFile(s.paths.GlobalConf)
	if err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "read nginx.conf", err)
	}
	httpDDdir := filepath.Dir(s.paths.DefaultConf)
	adjusted := replaceHttpDIncludeForTest(string(baseConf), httpDDdir, tmpDefaultPath)
	tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("nginx-test-%d.conf", time.Now().UnixNano()))
	if err := os.WriteFile(tmpPath, []byte(adjusted), 0o644); err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "write temp nginx conf", err)
	}
	defer os.Remove(tmpPath)

	if r, ok := s.runner.(*Runner); ok {
		return r.TestWithConfig(ctx, tmpPath)
	}
	return s.runner.Test(ctx)
}

// Reload runs nginx -t with automatic repair, ensures nginx is running, then reloads.
func (s *Service) Reload(ctx context.Context) error {
	if _, err := s.TestAndRepair(ctx); err != nil {
		return err
	}
	if err := s.EnsureRunning(ctx); err != nil {
		return err
	}
	return s.runner.Reload(ctx)
}

// EnsureRunning starts nginx when the master process is not running.
func (s *Service) EnsureRunning(ctx context.Context) error {
	if _, ok := s.runner.(*NoopReloadRunner); ok {
		return nil
	}
	r, ok := s.repairableRunner()
	if !ok {
		return nil
	}
	return r.Start(ctx)
}

// TestAndRepair runs nginx -t and applies safe automatic fixes.
func (s *Service) TestAndRepair(ctx context.Context) ([]RepairAction, error) {
	if r, ok := s.repairableRunner(); ok {
		return r.TestAndRepair(ctx, s.repairConfig())
	}
	if err := s.runner.Test(ctx); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *Service) repairableRunner() (*Runner, bool) {
	switch r := s.runner.(type) {
	case *Runner:
		return r, true
	case *NoopReloadRunner:
		return r.InnerRunner()
	default:
		return nil, false
	}
}

func (s *Service) repairConfig() RepairConfig {
	nginxDir := filepath.Dir(s.paths.GlobalConf)
	return RepairConfig{
		DefaultCert: filepath.Join(s.paths.SSLDefaultDir, "cert.pem"),
		DefaultKey:  filepath.Join(s.paths.SSLDefaultDir, "key.pem"),
		AllowPrefixes: []string{
			s.paths.SiteD,
			s.paths.ActiveD,
			s.paths.GlobalConf,
			s.paths.DefaultConf,
			nginxDir,
			filepath.Join(s.paths.Storage, "webconfig"),
			filepath.Join(s.paths.Storage, "nginx"),
		},
		MaxAttempts: 8,
	}
}

// BackupSiteConfig backs up site.d config to webconfig/backups/.
func (s *Service) BackupSiteConfig(ctx context.Context, domain string) (string, error) {
	return s.runner.BackupSiteConfig(ctx, domain)
}

// WriteSiteConfig writes site.d config without reload.
func (s *Service) WriteSiteConfig(ctx context.Context, domain, content string) error {
	return s.runner.WriteSiteConfig(ctx, domain, content)
}

// ReadSiteConfig reads site.d config.
func (s *Service) ReadSiteConfig(ctx context.Context, domain string) (string, error) {
	return s.runner.ReadSiteConfig(ctx, domain)
}

// UpdateSiteConfig backs up, writes, tests, and rolls back on test failure.
func (s *Service) UpdateSiteConfig(ctx context.Context, domain, content string) error {
	original, readErr := s.runner.ReadSiteConfig(ctx, domain)
	if readErr != nil {
		original = ""
	}

	if _, err := s.runner.BackupSiteConfig(ctx, domain); err != nil && readErr == nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "backup site config", err)
	}

	if err := s.runner.WriteSiteConfig(ctx, domain, content); err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "write site config", err)
	}

	if err := s.TestConfig(ctx, domain, content); err != nil {
		if original != "" {
			_ = s.runner.WriteSiteConfig(ctx, domain, original)
		}
		return err
	}
	return nil
}

// EnableSite creates active.d symlink to site.d config.
func (s *Service) EnableSite(ctx context.Context, domain string) error {
	if err := os.MkdirAll(s.paths.ActiveD, 0o755); err != nil {
		return fmt.Errorf("mkdir active.d: %w", err)
	}
	src := filepath.Join(s.paths.SiteD, domain+".conf")
	dst := filepath.Join(s.paths.ActiveD, domain+".conf")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("site config missing: %w", err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return nil
	}
	return os.Symlink(src, dst)
}

// DisableSite removes active.d symlink.
func (s *Service) DisableSite(ctx context.Context, domain string) error {
	dst := filepath.Join(s.paths.ActiveD, domain+".conf")
	if _, err := os.Lstat(dst); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Remove(dst)
}

// IsEnabled reports whether active.d symlink exists.
func (s *Service) IsEnabled(domain string) bool {
	dst := filepath.Join(s.paths.ActiveD, domain+".conf")
	_, err := os.Lstat(dst)
	return err == nil
}

// RemoveSiteConfig deletes site.d config file.
func (s *Service) RemoveSiteConfig(domain string) error {
	path := filepath.Join(s.paths.SiteD, domain+".conf")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ReadGlobalConfig reads nginx.conf.
func (s *Service) ReadGlobalConfig() (string, error) {
	data, err := os.ReadFile(s.paths.GlobalConf)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReadDefaultConfig reads http.d/default.conf.
func (s *Service) ReadDefaultConfig() (string, error) {
	data, err := os.ReadFile(s.paths.DefaultConf)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UpdateGlobalConfig tests and writes nginx.conf, rolling back on failure.
func (s *Service) UpdateGlobalConfig(ctx context.Context, content string) error {
	original, _ := s.ReadGlobalConfig()
	if err := s.TestRawConfig(ctx, content); err != nil {
		return err
	}
	if err := os.WriteFile(s.paths.GlobalConf, []byte(content), 0o644); err != nil {
		return err
	}
	if err := s.Reload(ctx); err != nil {
		if original != "" {
			_ = os.WriteFile(s.paths.GlobalConf, []byte(original), 0o644)
		}
		return err
	}
	return nil
}

// UpdateDefaultConfig tests and writes default server config.
func (s *Service) UpdateDefaultConfig(ctx context.Context, content string) error {
	original, _ := s.ReadDefaultConfig()
	if err := s.TestDefaultConfig(ctx, content); err != nil {
		return err
	}
	if err := os.WriteFile(s.paths.DefaultConf, []byte(content), 0o644); err != nil {
		return err
	}
	if err := s.Reload(ctx); err != nil {
		if original != "" {
			_ = os.WriteFile(s.paths.DefaultConf, []byte(original), 0o644)
		}
		return err
	}
	return nil
}

// DefaultSSLPaths returns default cert and key paths for a domain.
func (s *Service) DefaultSSLPaths(domain string) (cert, key string) {
	base := filepath.Join(s.paths.Storage, "webconfig/ssl/live", domain)
	return filepath.Join(base, "cert.pem"), filepath.Join(base, "key.pem")
}

// EnsureDomainSSL copies default SSL material into domain live directory.
func (s *Service) EnsureDomainSSL(domain string) error {
	srcCert := filepath.Join(s.paths.SSLDefaultDir, "cert.pem")
	srcKey := filepath.Join(s.paths.SSLDefaultDir, "key.pem")
	dstDir := filepath.Join(s.paths.Storage, "webconfig/ssl/live", domain)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	if err := copyFile(srcCert, filepath.Join(dstDir, "cert.pem")); err != nil {
		return err
	}
	return copyFile(srcKey, filepath.Join(dstDir, "key.pem"))
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// replaceHttpDIncludeForTest swaps the http.d glob include with an absolute temp default config path.
func replaceHttpDIncludeForTest(baseConf, httpDDdir, defaultConfigPath string) string {
	httpGlob := filepath.ToSlash(filepath.Join(httpDDdir, "*.conf"))
	tmpDefault := filepath.ToSlash(defaultConfigPath)
	if strings.Contains(baseConf, httpGlob) {
		return strings.Replace(baseConf, httpGlob, tmpDefault, 1)
	}
	legacyGlob := "/etc/nginx/http.d/*.conf"
	if strings.Contains(baseConf, legacyGlob) {
		return strings.Replace(baseConf, legacyGlob, tmpDefault, 1)
	}
	return strings.Replace(baseConf, "http.d/*.conf", tmpDefault, 1)
}

// replaceSiteIncludeForTest swaps the site.d glob include with an absolute temp config path.
func replaceSiteIncludeForTest(baseConf, siteD, siteConfigPath string) string {
	siteGlob := filepath.ToSlash(filepath.Join(siteD, "*.conf"))
	tmpSite := filepath.ToSlash(siteConfigPath)
	if strings.Contains(baseConf, siteGlob) {
		return strings.Replace(baseConf, siteGlob, tmpSite, 1)
	}
	// webconfig/nginx.conf ships with a fixed storage prefix; fall back when siteD differs (tests).
	legacyGlob := "/storage/webconfig/site.d/*.conf"
	if strings.Contains(baseConf, legacyGlob) {
		return strings.Replace(baseConf, legacyGlob, tmpSite, 1)
	}
	return strings.Replace(baseConf, "site.d/*.conf", tmpSite, 1)
}
