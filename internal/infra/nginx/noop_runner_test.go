package nginx_test

import (
	"context"
	"testing"

	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRunner struct {
	reloadCalled bool
}

func (s *stubRunner) Test(ctx context.Context) error          { return nil }
func (s *stubRunner) Reload(ctx context.Context) error        { s.reloadCalled = true; return nil }
func (s *stubRunner) WriteSiteConfig(ctx context.Context, domain, content string) error {
	return nil
}
func (s *stubRunner) ReadSiteConfig(ctx context.Context, domain string) (string, error) {
	return "", nil
}
func (s *stubRunner) BackupSiteConfig(ctx context.Context, domain string) (string, error) {
	return "", nil
}

func TestNoopReloadRunner_SkipsReload(t *testing.T) {
	inner := &stubRunner{}
	runner := nginx.NewNoopReloadRunner(inner)
	require.NoError(t, runner.Reload(context.Background()))
	assert.False(t, inner.reloadCalled)
}
