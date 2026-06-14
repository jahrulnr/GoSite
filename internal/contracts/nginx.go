package contracts

import "context"

// NginxRunner abstracts nginx config validation and reload operations.
type NginxRunner interface {
	Test(ctx context.Context) error
	Reload(ctx context.Context) error
	WriteSiteConfig(ctx context.Context, domain, content string) error
	ReadSiteConfig(ctx context.Context, domain string) (string, error)
	BackupSiteConfig(ctx context.Context, domain string) (string, error)
}
