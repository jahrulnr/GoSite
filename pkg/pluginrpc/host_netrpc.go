package pluginrpc

import (
	"errors"
	"net/rpc"

	hplugin "github.com/hashicorp/go-plugin"
)

// HostNetRPCPlugin is the host-side go-plugin adapter for tier-1 net/rpc plugins.
type HostNetRPCPlugin struct{}

func (HostNetRPCPlugin) Server(*hplugin.MuxBroker) (interface{}, error) {
	return nil, errors.New("pluginrpc: host-only client plugin")
}

func (HostNetRPCPlugin) Client(_ *hplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &netRPCClient{client: c}, nil
}

type netRPCClient struct {
	client *rpc.Client
}

func (c *netRPCClient) Validate(req ValidateRequest, resp *ValidateResponse) error {
	return c.client.Call("Plugin.Validate", req, resp)
}

func (c *netRPCClient) Health(req HealthRequest, resp *HealthResponse) error {
	return c.client.Call("Plugin.Health", req, resp)
}

func (c *netRPCClient) CallHook(req CallHookRequest, resp *CallHookResponse) error {
	return c.client.Call("Plugin.CallHook", req, resp)
}

func (c *netRPCClient) MigrateConfig(req MigrateConfigRequest, resp *MigrateConfigResponse) error {
	return c.client.Call("Plugin.MigrateConfig", req, resp)
}

// HostPluginMap returns the go-plugin client map for tier-1 net/rpc runtimes.
func HostPluginMap() map[string]hplugin.Plugin {
	return map[string]hplugin.Plugin{
		HandshakeMagic: HostNetRPCPlugin{},
	}
}
