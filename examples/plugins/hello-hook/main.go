// Command hello-hook is a minimal Tier 1 reference plugin for GoSite.
//
// It implements pluginrpc.Plugin over HashiCorp go-plugin (net/rpc) and
// declares no-op handlers for the events listed in its manifest.
//
// Build a plugin zip:
//
//	cd examples/plugins/hello-hook
//	make build
//
// The resulting dist/gosite-hello-hook.zip contains:
//
//	manifest.json
//	plugin/gosite     # the binary
//	plugin/validate   # the validate binary (any name works)
//
// Sign the zip with an ed25519 key and install it through the GoSite
// plugin API. Once enabled, the host's hook bus will invoke the plugin
// for every event listed in capabilities.hooks.
package main

import (
	"errors"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/jahrulnr/gosite/pkg/pluginrpc"
)

type helloPlugin struct{}

func (helloPlugin) Validate(req pluginrpc.ValidateRequest, resp *pluginrpc.ValidateResponse) error {
	resp.OK = true
	return nil
}

func (helloPlugin) Health(req pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	resp.OK = true
	resp.State = "ready"
	return nil
}

func (helloPlugin) CallHook(req pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
	resp.Status = "ok"
	return nil
}

func (helloPlugin) MigrateConfig(req pluginrpc.MigrateConfigRequest, resp *pluginrpc.MigrateConfigResponse) error {
	resp.OK = true
	resp.MigratedConfig = req.CurrentConfig
	return nil
}

func main() {
	pluginMap := map[string]plugin.Plugin{
		pluginrpc.HandshakeMagic: &rpcPlugin{impl: helloPlugin{}},
	}
	hc := plugin.HandshakeConfig{
		ProtocolVersion:  uint(1),
		MagicCookieKey:   "gosite",
		MagicCookieValue: pluginrpc.HandshakeMagic,
	}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: hc,
		Plugins:         pluginMap,
	})
}

// rpcPlugin serves the pluginrpc.Plugin interface over HashiCorp's
// net/rpc protocol. The Server method returns an object whose exported
// methods are dispatched by the host via net/rpc.
type rpcPlugin struct {
	impl pluginrpc.Plugin
}

func (p *rpcPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &rpcServer{impl: p.impl}, nil
}

func (p *rpcPlugin) Client(*plugin.MuxBroker, *rpc.Client) (interface{}, error) {
	return nil, errors.New("client not supported for server-only plugin")
}

type rpcServer struct {
	impl pluginrpc.Plugin
}

func (s *rpcServer) Validate(req pluginrpc.ValidateRequest, resp *pluginrpc.ValidateResponse) error {
	return s.impl.Validate(req, resp)
}

func (s *rpcServer) Health(req pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	return s.impl.Health(req, resp)
}

func (s *rpcServer) CallHook(req pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
	return s.impl.CallHook(req, resp)
}

func (s *rpcServer) MigrateConfig(req pluginrpc.MigrateConfigRequest, resp *pluginrpc.MigrateConfigResponse) error {
	return s.impl.MigrateConfig(req, resp)
}
