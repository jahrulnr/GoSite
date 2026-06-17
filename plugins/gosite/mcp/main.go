// GoSite MCP tier-1 runtime (minimal lifecycle hooks).
package main

import (
	"log"

	"github.com/jahrulnr/gosite/pkg/pluginrpc"
	"github.com/jahrulnr/gosite/plugins/_templates/_shared/rpcplugin"
)

type mcpPlugin struct{}

func (mcpPlugin) Validate(_ pluginrpc.ValidateRequest, resp *pluginrpc.ValidateResponse) error {
	resp.OK = true
	return nil
}

func (mcpPlugin) Health(_ pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	resp.OK = true
	resp.State = "ready"
	return nil
}

func (mcpPlugin) CallHook(req pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
	log.Printf("gosite/mcp: event=%s", req.EventName)
	resp.Status = "ok"
	return nil
}

func (mcpPlugin) MigrateConfig(req pluginrpc.MigrateConfigRequest, resp *pluginrpc.MigrateConfigResponse) error {
	resp.OK = true
	resp.MigratedConfig = req.CurrentConfig
	return nil
}

func main() {
	rpcplugin.Serve(mcpPlugin{})
}
