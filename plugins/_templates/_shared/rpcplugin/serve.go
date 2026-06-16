// Package rpcplugin bridges a pluginrpc.Plugin implementation to HashiCorp
// go-plugin net/rpc. Copy this package into your plugin repo or import it
// from the GoSite monorepo when developing in-tree.
package rpcplugin

import (
	"errors"
	"net/rpc"

	hplugin "github.com/hashicorp/go-plugin"
	"github.com/jahrulnr/gosite/pkg/pluginrpc"
)

// Serve blocks and serves impl over the canonical GoSite handshake.
func Serve(impl pluginrpc.Plugin) {
	pluginMap := map[string]hplugin.Plugin{
		pluginrpc.HandshakeMagic: &bridge{impl: impl},
	}
	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: hplugin.HandshakeConfig{
			ProtocolVersion:  uint(1),
			MagicCookieKey:   "gosite",
			MagicCookieValue: pluginrpc.HandshakeMagic,
		},
		Plugins: pluginMap,
	})
}

type bridge struct{ impl pluginrpc.Plugin }

func (b *bridge) Server(*hplugin.MuxBroker) (interface{}, error) {
	return &server{impl: b.impl}, nil
}

func (b *bridge) Client(*hplugin.MuxBroker, *rpc.Client) (interface{}, error) {
	return nil, errors.New("rpcplugin: server-only plugin")
}

type server struct{ impl pluginrpc.Plugin }

func (s *server) Validate(req pluginrpc.ValidateRequest, resp *pluginrpc.ValidateResponse) error {
	return s.impl.Validate(req, resp)
}

func (s *server) Health(req pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	return s.impl.Health(req, resp)
}

func (s *server) CallHook(req pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
	return s.impl.CallHook(req, resp)
}

func (s *server) MigrateConfig(req pluginrpc.MigrateConfigRequest, resp *pluginrpc.MigrateConfigResponse) error {
	return s.impl.MigrateConfig(req, resp)
}
