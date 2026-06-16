package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	hplugin "github.com/hashicorp/go-plugin"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/plugin/hookbus"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/jahrulnr/gosite/pkg/pluginrpc"
)

// GoPluginClient is the host-side gRPC stub exposed by HashiCorp go-plugin.
// It is kept as a small interface so unit tests can supply a fake.
type GoPluginClient interface {
	pluginrpc.Plugin
}

// GoPluginClientFactory starts a plugin subprocess and returns a typed
// client. The default factory uses HashiCorp go-plugin; tests can swap it
// for a deterministic in-process implementation.
type GoPluginClientFactory func(ctx context.Context, artifactPath, pluginName string) (GoPluginClient, func() error, error)

// DefaultGoPluginClientFactory launches a tier-1 plugin via HashiCorp
// go-plugin using net/rpc over the standard handshake. The plugin binary
// is expected to import gosite/pkg/pluginrpc and call pluginrpc.Serve at
// init() time. The host dispenses the client by the canonical magic
// string and casts to pluginrpc.Plugin.
func DefaultGoPluginClientFactory(ctx context.Context, artifactPath, pluginName string) (GoPluginClient, func() error, error) {
	if pluginName == "" {
		pluginName = "gosite"
	}
	commandPath, args, err := resolvePluginCommand(filepath.Dir(artifactPath), "plugin/"+pluginName)
	if err != nil {
		return nil, nil, err
	}
	if _, statErr := os.Stat(commandPath); statErr != nil {
		return nil, nil, apperror.New(apperror.CodePluginInvalid, "plugin entrypoint not found: "+commandPath)
	}
	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	type startResult struct {
		client *hplugin.Client
		impl   pluginrpc.Plugin
		err    error
	}
	resultCh := make(chan startResult, 1)
	go func() {
		client := hplugin.NewClient(&hplugin.ClientConfig{
			HandshakeConfig: hplugin.HandshakeConfig{
				ProtocolVersion:  uint(1),
				MagicCookieKey:   "gosite",
				MagicCookieValue: pluginrpc.HandshakeMagic,
			},
			AllowedProtocols: []hplugin.Protocol{hplugin.ProtocolNetRPC},
			Cmd:              exec.Command(commandPath, args...),
			SyncStdout:       os.Stderr,
			SyncStderr:       os.Stderr,
		})
		rpcClient, err := client.Client()
		if err != nil {
			client.Kill()
			resultCh <- startResult{err: err}
			return
		}
		raw, err := rpcClient.Dispense(pluginrpc.HandshakeMagic)
		if err != nil {
			client.Kill()
			resultCh <- startResult{err: err}
			return
		}
		impl, ok := raw.(pluginrpc.Plugin)
		if !ok {
			client.Kill()
			resultCh <- startResult{err: errors.New("plugin did not implement pluginrpc.Plugin")}
			return
		}
		resultCh <- startResult{client: client, impl: impl}
	}()
	var res startResult
	select {
	case res = <-resultCh:
	case <-startCtx.Done():
		return nil, nil, apperror.Wrap(apperror.CodePluginOperation, "plugin handshake timeout", startCtx.Err())
	}
	if res.err != nil {
		return nil, nil, apperror.Wrap(apperror.CodePluginOperation, "plugin handshake failed", res.err)
	}
	kill := func() error { res.client.Kill(); return nil }
	return res.impl, kill, nil
}

// GoPluginRuntimeManager replaces ProcessRuntimeManager for tier-1
// plugins. It launches a HashiCorp go-plugin subprocess per enabled
// version and exposes the same RuntimeManager contract to the lifecycle
// service. Tier-0 plugins fall through to no-op (webhooks use a different
// path) so this manager is safe as a default.
//
// Hook calls reach the plugin via the gRPC stub and are routed through
// HookCallerAdapter into the hook bus.
type GoPluginRuntimeManager struct {
	mu      sync.Mutex
	clients map[string]managedPlugin
	factory GoPluginClientFactory
	startTO time.Duration
}

type managedPlugin struct {
	client  GoPluginClient
	kill    func() error
	started time.Time
}

// NewGoPluginRuntimeManager returns a tier-1 runtime manager using the
// default HashiCorp go-plugin client factory.
func NewGoPluginRuntimeManager() *GoPluginRuntimeManager {
	return &GoPluginRuntimeManager{
		clients: map[string]managedPlugin{},
		factory: DefaultGoPluginClientFactory,
		startTO: 15 * time.Second,
	}
}

// NewGoPluginRuntimeManagerWithFactory is the test-friendly constructor.
func NewGoPluginRuntimeManagerWithFactory(factory GoPluginClientFactory) *GoPluginRuntimeManager {
	mgr := NewGoPluginRuntimeManager()
	if factory != nil {
		mgr.factory = factory
	}
	return mgr
}

// Start launches the plugin subprocess and dispenses the gRPC client.
func (m *GoPluginRuntimeManager) Start(ctx context.Context, plugin sqlite.PluginVersion) error {
	manifest := manifestFromRecord(plugin)
	pluginName := manifest.Entrypoints["runtime"].Command
	if pluginName == "" {
		pluginName = "gosite"
	}
	factoryCtx, cancel := context.WithTimeout(ctx, m.startTO)
	defer cancel()
	client, kill, err := m.factory(factoryCtx, plugin.ArtifactPath, pluginName)
	if err != nil {
		return err
	}
	key := plugin.PluginID + "@" + plugin.Version
	m.mu.Lock()
	if existing, ok := m.clients[key]; ok {
		_ = existing.kill()
	}
	m.clients[key] = managedPlugin{client: client, kill: kill, started: time.Now()}
	m.mu.Unlock()
	return nil
}

// Stop terminates the plugin subprocess and removes the client from the
// active set. Safe to call multiple times.
func (m *GoPluginRuntimeManager) Stop(_ context.Context, plugin sqlite.PluginVersion) error {
	key := plugin.PluginID + "@" + plugin.Version
	m.mu.Lock()
	managed, ok := m.clients[key]
	delete(m.clients, key)
	m.mu.Unlock()
	if !ok {
		return nil
	}
	return managed.kill()
}

// EnsureStopped is an alias for Stop.
func (m *GoPluginRuntimeManager) EnsureStopped(ctx context.Context, plugin sqlite.PluginVersion) error {
	return m.Stop(ctx, plugin)
}

// Health pings a single plugin. Returns an error if the plugin is not
// running, the gRPC call fails, or the plugin reports an unhealthy state.
func (m *GoPluginRuntimeManager) Health(ctx context.Context, plugin sqlite.PluginVersion) error {
	key := plugin.PluginID + "@" + plugin.Version
	m.mu.Lock()
	managed, ok := m.clients[key]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("plugin %s not running", key)
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	var resp pluginrpc.HealthResponse
	go func() {
		done <- managed.client.Health(pluginrpc.HealthRequest{PluginID: plugin.PluginID, Version: plugin.Version}, &resp)
	}()
	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-callCtx.Done():
		return apperror.New(apperror.CodePluginOperation, "plugin health timeout")
	}
	if !resp.OK {
		return apperror.New(apperror.CodePluginOperation, "plugin unhealthy: "+resp.Error)
	}
	return nil
}

// HookCallerAdapter routes hook bus events to running tier-1 plugin gRPC
// stubs. Implementations of hookbus.HookCaller can be plugged into the bus
// configuration; when no client is running the adapter reports a clear
// error so the breaker can engage.
type HookCallerAdapter struct {
	manager *GoPluginRuntimeManager
}

// NewHookCallerAdapter wires GoPluginRuntimeManager into the hook bus.
func NewHookCallerAdapter(m *GoPluginRuntimeManager) *HookCallerAdapter {
	return &HookCallerAdapter{manager: m}
}

// CallHook dispatches one event to one plugin target over the gRPC stub.
func (a *HookCallerAdapter) CallHook(_ context.Context, target hookbus.Target, eventName string, payload json.RawMessage) (json.RawMessage, error) {
	key := target.PluginID + "@" + target.Version
	a.manager.mu.Lock()
	managed, ok := a.manager.clients[key]
	a.manager.mu.Unlock()
	if !ok {
		return nil, errors.New("plugin subprocess not running")
	}
	req := pluginrpc.CallHookRequest{
		PluginID:    target.PluginID,
		Version:     target.Version,
		EventName:   eventName,
		PayloadJSON: string(payload),
	}
	var resp pluginrpc.CallHookResponse
	if err := managed.client.CallHook(req, &resp); err != nil {
		return nil, err
	}
	if resp.Status == "error" {
		return nil, errors.New(resp.Error)
	}
	if resp.SideEffect == "" {
		return json.RawMessage("{}"), nil
	}
	return json.RawMessage(resp.SideEffect), nil
}

// ValidateConfig invokes a tier-1 plugin's MigrateConfig RPC. Used by the
// switch flow to validate config compatibility before disable.
func (m *GoPluginRuntimeManager) ValidateConfig(ctx context.Context, current, next sqlite.PluginVersion, currentConfig string) (string, error) {
	key := current.PluginID + "@" + current.Version
	m.mu.Lock()
	managed, ok := m.clients[key]
	m.mu.Unlock()
	if !ok {
		return currentConfig, nil
	}
	req := pluginrpc.MigrateConfigRequest{
		Current:       snapshotFromRecord(current),
		Next:          snapshotFromRecord(next),
		CurrentConfig: currentConfig,
	}
	var resp pluginrpc.MigrateConfigResponse
	if err := managed.client.MigrateConfig(req, &resp); err != nil {
		return "", err
	}
	if !resp.OK {
		return "", apperror.New(apperror.CodePluginOperation, "config migration rejected: "+resp.Error)
	}
	if resp.MigratedConfig != "" {
		return resp.MigratedConfig, nil
	}
	return currentConfig, nil
}

func snapshotFromRecord(p sqlite.PluginVersion) pluginrpc.PluginManifestSnapshot {
	return pluginrpc.PluginManifestSnapshot{
		PluginID:         p.PluginID,
		Version:          p.Version,
		Tier:             p.Tier,
		APIVersion:       p.APIVersion,
		MinGoSiteVersion: p.MinGoSiteVersion,
		RPCVersion:       p.RPCVersion,
		ConfigVersion:    p.ConfigVersion,
	}
}
