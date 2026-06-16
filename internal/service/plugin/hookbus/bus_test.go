package hookbus_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin/hookbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type callerStub struct {
	mu    sync.Mutex
	errs  map[string]error
	calls []string
}

func (c *callerStub) CallHook(_ context.Context, target hookbus.Target, eventName string, _ json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := target.PluginID + "@" + target.Version
	c.calls = append(c.calls, key+":"+eventName)
	return json.RawMessage(`{}`), c.errs[key]
}

func (c *callerStub) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

func manifest(hooks []string, isolation string) string {
	hooksJSON, _ := json.Marshal(hooks)
	return `{"capabilities":{"hooks":` + string(hooksJSON) + `,"hookIsolation":"` + isolation + `"}}`
}

func TestDispatchStrictStopsOnFirstError(t *testing.T) {
	t.Parallel()

	caller := &callerStub{errs: map[string]error{"acme/bad@1.0.0": errors.New("blocked")}}
	bus := hookbus.New(hookbus.Config{Caller: caller, HookTimeout: time.Second})
	require.NoError(t, bus.Refresh(context.Background(), []sqlite.PluginVersion{
		{PluginID: "acme/bad", Version: "1.0.0", ManifestJSON: manifest([]string{"nginx.before_reload"}, "sequential")},
		{PluginID: "acme/next", Version: "1.0.0", ManifestJSON: manifest([]string{"nginx.before_reload"}, "sequential")},
	}))

	result, err := bus.Dispatch(context.Background(), "nginx.before_reload", map[string]string{"source": "test"})

	require.Error(t, err)
	assert.True(t, result.Strict)
	assert.Len(t, result.Calls, 1)
	assert.Equal(t, []string{"acme/bad@1.0.0:nginx.before_reload"}, caller.snapshot())
}

func TestDispatchLenientContinuesAfterError(t *testing.T) {
	t.Parallel()

	caller := &callerStub{errs: map[string]error{"acme/bad@1.0.0": errors.New("soft fail")}}
	bus := hookbus.New(hookbus.Config{Caller: caller, StrictEvents: map[string]bool{"nginx.after_reload": false}, HookTimeout: time.Second})
	require.NoError(t, bus.Refresh(context.Background(), []sqlite.PluginVersion{
		{PluginID: "acme/bad", Version: "1.0.0", ManifestJSON: manifest([]string{"nginx.after_reload"}, "sequential")},
		{PluginID: "acme/next", Version: "1.0.0", ManifestJSON: manifest([]string{"nginx.after_reload"}, "sequential")},
	}))

	result, err := bus.Dispatch(context.Background(), "nginx.after_reload", map[string]string{"source": "test"})

	require.NoError(t, err)
	assert.False(t, result.Strict)
	assert.Len(t, result.Calls, 2)
	assert.Len(t, result.Warnings, 1)
	assert.Equal(t, []string{"acme/bad@1.0.0:nginx.after_reload", "acme/next@1.0.0:nginx.after_reload"}, caller.snapshot())
}
