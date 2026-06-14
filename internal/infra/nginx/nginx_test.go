package nginx_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderStatic_SubstitutesPlaceholders(t *testing.T) {
	t.Parallel()

	tpl := testutil.ConfigPath("webconfig/site.conf")
	out, err := nginx.RenderStatic(tpl, nginx.SiteTemplateData{
		Domain:   "site.test",
		Path:     "/www/site",
		SSLCert:  "/ssl/cert.pem",
		SSLKey:   "/ssl/key.pem",
	})
	require.NoError(t, err)
	assert.Contains(t, out, "server_name site.test;")
	assert.Contains(t, out, "root /www/site;")
	assert.Contains(t, out, "ssl_certificate /ssl/cert.pem;")
}

func TestRenderProxy_IncludesUpstream(t *testing.T) {
	t.Parallel()

	tpl := testutil.ConfigPath("webconfig/site-proxy.conf")
	out, err := nginx.RenderProxy(tpl, nginx.SiteTemplateData{
		Domain:   "proxy.test",
		Upstream: "http://127.0.0.1:9000",
		SSLCert:  "/ssl/cert.pem",
		SSLKey:   "/ssl/key.pem",
	})
	require.NoError(t, err)
	assert.Contains(t, out, "proxy_pass http://127.0.0.1:9000;")
}

func TestUpdateSSLDirectives_ReplacesPaths(t *testing.T) {
	t.Parallel()

	in := "ssl_certificate /old/cert.pem;\nssl_certificate_key /old/key.pem;"
	out := nginx.UpdateSSLDirectives(in, "/new/cert.pem", "/new/key.pem")
	assert.Contains(t, out, "ssl_certificate /new/cert.pem;")
	assert.Contains(t, out, "ssl_certificate_key /new/key.pem;")
}

func TestParseCertPaths(t *testing.T) {
	t.Parallel()

	cfg := "ssl_certificate /a/cert.pem;\nssl_certificate_key /a/key.pem;"
	cert, key, ok := nginx.ParseCertPaths(cfg)
	assert.True(t, ok)
	assert.Equal(t, "/a/cert.pem", cert)
	assert.Equal(t, "/a/key.pem", key)
}

func TestNginxService_EnableDisableSite(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	domain := "enable.test"
	content := testutil.SampleNginxSiteConfig
	require.NoError(t, stack.Runner.WriteSiteConfig(ctx, domain, content))

	require.NoError(t, stack.Nginx.EnableSite(ctx, domain))
	activePath := filepath.Join(stack.Nginx.Paths().ActiveD, domain+".conf")
	_, err := os.Lstat(activePath)
	require.NoError(t, err)

	require.NoError(t, stack.Nginx.DisableSite(ctx, domain))
	_, err = os.Lstat(activePath)
	require.Error(t, err)
}

func TestNginxConfig_BackupCreated(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	domain := "backup.test"
	require.NoError(t, stack.Runner.WriteSiteConfig(ctx, domain, testutil.SampleNginxSiteConfig))

	path, err := stack.Nginx.BackupSiteConfig(ctx, domain)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(path, stack.Nginx.Paths().Backups))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "example.test")
}

func TestNginxService_UpdateSiteConfig_RollbackOnTestFail(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	domain := "rollback.test"
	original := testutil.SampleNginxSiteConfig
	require.NoError(t, stack.Runner.WriteSiteConfig(ctx, domain, original))

	stack.Cmd.NginxTestFail = true
	bad := "server { invalid directive; }"
	err := stack.Nginx.UpdateSiteConfig(ctx, domain, bad)
	require.Error(t, err)

	got, err := stack.Runner.ReadSiteConfig(ctx, domain)
	require.NoError(t, err)
	assert.Equal(t, original, got)
	stack.Cmd.NginxTestFail = false
}

func TestRunner_TestAndReload(t *testing.T) {
	stack := testutil.SetupTestStack(t)
	ctx := context.Background()

	require.NoError(t, stack.Runner.Test(ctx))
	require.NoError(t, stack.Runner.Reload(ctx))
	assert.NotZero(t, stack.Cmd.Calls)
}
