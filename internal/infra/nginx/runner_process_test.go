package nginx_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_IsRunning_falseWithoutPIDFile(t *testing.T) {
	t.Parallel()
	runner := nginx.NewRunner(testutil.NewMockCommander(), nginx.RunnerConfig{
		PIDFile: filepath.Join(t.TempDir(), "missing.pid"),
	})
	assert.False(t, runner.IsRunning())
}

func TestRunner_Start_invokesNginxWhenNotRunning(t *testing.T) {
	t.Parallel()
	cmd := testutil.NewMockCommander()
	runner := nginx.NewRunner(cmd, nginx.RunnerConfig{
		PIDFile: filepath.Join(t.TempDir(), "missing.pid"),
		NginxConf: "/etc/nginx/nginx.conf",
	})
	require.NoError(t, runner.Start(context.Background()))
	calls := cmd.SnapshotCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "nginx", calls[0].Name)
	assert.Equal(t, []string{"-c", "/etc/nginx/nginx.conf"}, calls[0].Args)
}

func TestService_EnsureRunning_noopInLocal(t *testing.T) {
	t.Parallel()
	cfg := config.Config{AppEnv: "local", Storage: t.TempDir()}
	svc := nginx.NewServiceFromConfig(cfg, testutil.NewMockCommander())
	require.NoError(t, svc.EnsureRunning(context.Background()))
}
