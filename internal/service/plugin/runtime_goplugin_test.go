package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin/hookbus"
	"github.com/jahrulnr/gosite/pkg/pluginrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ = errors.New

type fakePlugin struct {
	mu          sync.Mutex
	hooks       []string
	healthOK    bool
	migrateOut  string
	migrateErr  error
	hookErr     error
	failNextHook bool
}

func (f *fakePlugin) Validate(req pluginrpc.ValidateRequest, resp *pluginrpc.ValidateResponse) error {
	resp.OK = true
	return nil
}

func (f *fakePlugin) Health(req pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	resp.OK = f.healthOK
	resp.State = "ready"
	return nil
}

func (f *fakePlugin) CallHook(req pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
	f.mu.Lock()
	f.hooks = append(f.hooks, req.EventName)
	fail := f.failNextHook
	f.failNextHook = false
	err := f.hookErr
	f.mu.Unlock()
	if err != nil {
		resp.Status = "error"
		resp.Error = err.Error()
		return nil
	}
	if fail {
		resp.Status = "error"
		resp.Error = "rejected"
		return nil
	}
	resp.Status = "ok"
	return nil
}

func (f *fakePlugin) MigrateConfig(req pluginrpc.MigrateConfigRequest, resp *pluginrpc.MigrateConfigResponse) error {
	if f.migrateErr != nil {
		resp.OK = false
		resp.Error = f.migrateErr.Error()
		return nil
	}
	resp.OK = true
	resp.MigratedConfig = f.migrateOut
	return nil
}

func newFakeFactory(p *fakePlugin) GoPluginClientFactory {
	return func(ctx context.Context, artifactPath, pluginName string) (GoPluginClient, func() error, error) {
		return p, func() error { return nil }, nil
	}
}

func samplePlugin() sqlite.PluginVersion {
	return sqlite.PluginVersion{
		PluginID:     "acme/hello",
		Version:      "1.0.0",
		Tier:         1,
		ManifestJSON: `{"entrypoints":{"runtime":{"command":"gosite"}}}`,
	}
}

func TestGoPluginRuntimeManagerStartAndStop(t *testing.T) {
	t.Parallel()
	plugin := &fakePlugin{healthOK: true}
	mgr := NewGoPluginRuntimeManagerWithFactory(newFakeFactory(plugin))
	ctx := context.Background()
	record := samplePlugin()
	record.ArtifactPath = "/tmp/gosite-hello.zip"

	require.NoError(t, mgr.Start(ctx, record))
	require.NoError(t, mgr.Health(ctx, record))
	require.NoError(t, mgr.Stop(ctx, record))
	require.Error(t, mgr.Health(ctx, record), "health after stop should fail")
}

func TestGoPluginRuntimeManagerUsesManifestRuntimeCommand(t *testing.T) {
	t.Parallel()
	var gotCommand string
	mgr := NewGoPluginRuntimeManagerWithFactory(func(ctx context.Context, artifactPath, command string) (GoPluginClient, func() error, error) {
		gotCommand = command
		return &fakePlugin{healthOK: true}, func() error { return nil }, nil
	})
	record := samplePlugin()
	record.ManifestJSON = `{"entrypoints":{"runtime":{"command":"plugin/gosite"}}}`
	record.ArtifactPath = "/tmp/gosite-hello.zip"
	require.NoError(t, mgr.Start(context.Background(), record))
	assert.Equal(t, "plugin/gosite", gotCommand)
}

func TestGoPluginRuntimeManagerHookCaller(t *testing.T) {
	t.Parallel()
	plugin := &fakePlugin{healthOK: true}
	mgr := NewGoPluginRuntimeManagerWithFactory(newFakeFactory(plugin))
	record := samplePlugin()
	record.ArtifactPath = "/tmp/gosite-hello.zip"
	require.NoError(t, mgr.Start(context.Background(), record))
	adapter := NewHookCallerAdapter(mgr)

	target := hookbus.Target{PluginID: record.PluginID, Version: record.Version, Tier: record.Tier, ManifestJSON: record.ManifestJSON}
	payload, _ := json.Marshal(map[string]string{"event": "test"})
	_, err := adapter.CallHook(context.Background(), target, "nginx.before_reload", payload)
	require.NoError(t, err)
	assert.Equal(t, []string{"nginx.before_reload"}, plugin.hooks)
}

func TestGoPluginRuntimeManagerHookCallerNotRunning(t *testing.T) {
	t.Parallel()
	mgr := NewGoPluginRuntimeManagerWithFactory(newFakeFactory(&fakePlugin{healthOK: true}))
	adapter := NewHookCallerAdapter(mgr)
	target := hookbus.Target{PluginID: "missing", Version: "1.0.0"}
	_, err := adapter.CallHook(context.Background(), target, "nginx.before_reload", []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestGoPluginRuntimeManagerValidateConfigPassThrough(t *testing.T) {
	t.Parallel()
	plugin := &fakePlugin{healthOK: true, migrateOut: `{"foo":"bar"}`}
	mgr := NewGoPluginRuntimeManagerWithFactory(newFakeFactory(plugin))
	current := samplePlugin()
	next := samplePlugin()
	next.Version = "2.0.0"
	require.NoError(t, mgr.Start(context.Background(), current))
	out, err := mgr.ValidateConfig(context.Background(), current, next, `{"foo":"baz"}`)
	require.NoError(t, err)
	assert.Equal(t, `{"foo":"bar"}`, out)
}

func TestGoPluginRuntimeManagerValidateConfigFailure(t *testing.T) {
	t.Parallel()
	plugin := &fakePlugin{healthOK: true, migrateErr: errors.New("schema mismatch")}
	mgr := NewGoPluginRuntimeManagerWithFactory(newFakeFactory(plugin))
	current := samplePlugin()
	next := samplePlugin()
	next.Version = "2.0.0"
	require.NoError(t, mgr.Start(context.Background(), current))
	_, err := mgr.ValidateConfig(context.Background(), current, next, `{}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config migration rejected")
}

func TestGoPluginRuntimeManagerHealthTimeout(t *testing.T) {
	t.Parallel()
	plugin := &fakePlugin{healthOK: true}
	mgr := NewGoPluginRuntimeManagerWithFactory(newFakeFactory(plugin))
	record := samplePlugin()
	record.ArtifactPath = "/tmp/gosite-hello.zip"
	// Override health to block by never returning; we can simulate by
	// swapping the client for one that hangs.
	mgr.clients[record.PluginID+"@"+record.Version] = managedPlugin{
		client: &blockingPlugin{},
		kill:   func() error { return nil },
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := mgr.Health(ctx, record)
	require.Error(t, err)
}

// blockingPlugin blocks until ctx is done.
type blockingPlugin struct{ fakePlugin }

func (b *blockingPlugin) Health(req pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	time.Sleep(500 * time.Millisecond)
	return nil
}
