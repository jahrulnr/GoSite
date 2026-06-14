package nginx

import (
	"context"

	"github.com/jahrulnr/gosite/internal/contracts"
)

// NoopReloadRunner delegates nginx operations to an inner runner but skips Reload in local dev.
type NoopReloadRunner struct {
	inner contracts.NginxRunner
}

// NewNoopReloadRunner wraps runner so Reload is a no-op.
func NewNoopReloadRunner(inner contracts.NginxRunner) *NoopReloadRunner {
	return &NoopReloadRunner{inner: inner}
}

func (n *NoopReloadRunner) Test(ctx context.Context) error {
	return n.inner.Test(ctx)
}

func (n *NoopReloadRunner) Reload(ctx context.Context) error {
	return nil
}

func (n *NoopReloadRunner) WriteSiteConfig(ctx context.Context, domain, content string) error {
	return n.inner.WriteSiteConfig(ctx, domain, content)
}

func (n *NoopReloadRunner) ReadSiteConfig(ctx context.Context, domain string) (string, error) {
	return n.inner.ReadSiteConfig(ctx, domain)
}

func (n *NoopReloadRunner) BackupSiteConfig(ctx context.Context, domain string) (string, error) {
	return n.inner.BackupSiteConfig(ctx, domain)
}
