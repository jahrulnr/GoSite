package nginx_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/infra/nginx"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin/hookbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type hookCallerStub struct {
	err   error
	calls []string
}

func (c *hookCallerStub) CallHook(_ context.Context, target hookbus.Target, eventName string, _ json.RawMessage) (json.RawMessage, error) {
	c.calls = append(c.calls, target.PluginID+"@"+target.Version+":"+eventName)
	return json.RawMessage(`{}`), c.err
}

func TestNginxReloadStrictHookFailureBlocksReload(t *testing.T) {
	t.Parallel()

	caller := &hookCallerStub{err: errors.New("blocked by plugin")}
	bus := hookbus.New(hookbus.Config{Caller: caller, HookTimeout: time.Second})
	require.NoError(t, bus.Refresh(context.Background(), []sqlite.PluginVersion{
		{
			PluginID:     "acme/guard",
			Version:      "1.0.0",
			ManifestJSON: `{"capabilities":{"hooks":["nginx.before_reload"],"hookIsolation":"sequential"}}`,
		},
	}))
	runner := &stubRunner{}
	svc := nginx.NewService(runner, nil, nginx.Paths{}, nginx.WithHookBus(bus))

	err := svc.Reload(context.Background())

	require.Error(t, err)
	assert.False(t, runner.reloadCalled)
	assert.Equal(t, []string{"acme/guard@1.0.0:nginx.before_reload"}, caller.calls)
}
