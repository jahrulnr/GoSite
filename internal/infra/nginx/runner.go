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

// RunnerConfig holds filesystem paths for nginx operations.
type RunnerConfig struct {
	SiteDDir    string
	BackupsDir  string
	NginxConf   string
	NginxBin    string
}

// Runner implements contracts.NginxRunner using a CommandRunner for exec.
type Runner struct {
	cmd  contracts.CommandRunner
	cfg  RunnerConfig
}

// NewRunner returns a nginx runner backed by cmd.
func NewRunner(cmd contracts.CommandRunner, cfg RunnerConfig) *Runner {
	if cfg.NginxBin == "" {
		cfg.NginxBin = "nginx"
	}
	return &Runner{cmd: cmd, cfg: cfg}
}

// Test validates the main nginx configuration.
func (r *Runner) Test(ctx context.Context) error {
	result, err := r.cmd.Run(ctx, r.cfg.NginxBin, "-t", "-c", r.cfg.NginxConf)
	if err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "nginx test failed", err)
	}
	if result.ExitCode != 0 || !strings.Contains(result.Stdout+result.Stderr, "syntax is ok") {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(result.Stdout)
		}
		return apperror.Wrap(apperror.CodeNginxTestFailed, msg, nil)
	}
	return nil
}

// TestWithConfig validates nginx using a temporary config file.
func (r *Runner) TestWithConfig(ctx context.Context, confPath string) error {
	result, err := r.cmd.Run(ctx, r.cfg.NginxBin, "-t", "-c", confPath)
	if err != nil {
		return apperror.Wrap(apperror.CodeNginxTestFailed, "nginx test failed", err)
	}
	if result.ExitCode != 0 || !strings.Contains(result.Stdout+result.Stderr, "syntax is ok") {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(result.Stdout)
		}
		return apperror.Wrap(apperror.CodeNginxTestFailed, msg, nil)
	}
	return nil
}

// Reload reloads nginx configuration.
func (r *Runner) Reload(ctx context.Context) error {
	result, err := r.cmd.Run(ctx, r.cfg.NginxBin, "-s", "reload")
	if err != nil {
		return apperror.Wrap(apperror.CodeNginxReloadFailed, "nginx reload failed", err)
	}
	if result.ExitCode != 0 {
		msg := strings.TrimSpace(result.Stderr)
		if msg == "" {
			msg = "nginx reload failed"
		}
		return apperror.Wrap(apperror.CodeNginxReloadFailed, msg, nil)
	}
	return nil
}

// WriteSiteConfig writes site.d/{domain}.conf.
func (r *Runner) WriteSiteConfig(ctx context.Context, domain, content string) error {
	if err := os.MkdirAll(r.cfg.SiteDDir, 0o755); err != nil {
		return fmt.Errorf("mkdir site.d: %w", err)
	}
	path := filepath.Join(r.cfg.SiteDDir, domain+".conf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write site config: %w", err)
	}
	return nil
}

// ReadSiteConfig reads site.d/{domain}.conf.
func (r *Runner) ReadSiteConfig(ctx context.Context, domain string) (string, error) {
	path := filepath.Join(r.cfg.SiteDDir, domain+".conf")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read site config: %w", err)
	}
	return string(data), nil
}

// BackupSiteConfig copies site config to webconfig/backups/.
func (r *Runner) BackupSiteConfig(ctx context.Context, domain string) (string, error) {
	src := filepath.Join(r.cfg.SiteDDir, domain+".conf")
	data, err := os.ReadFile(src)
	if err != nil {
		return "", fmt.Errorf("read site config for backup: %w", err)
	}
	if err := os.MkdirAll(r.cfg.BackupsDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir backups: %w", err)
	}
	name := fmt.Sprintf("%s-%d.conf", domain, time.Now().Unix())
	dest := filepath.Join(r.cfg.BackupsDir, name)
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}
	return dest, nil
}
