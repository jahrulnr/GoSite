// Minimal Tier 1 go-plugin template.
//
// Build: make build
package main

import (
	"log"

	"github.com/jahrulnr/gosite/pkg/pluginrpc"
	"github.com/jahrulnr/gosite/plugins/_templates/_shared/rpcplugin"
)

type minimalPlugin struct{}

func (minimalPlugin) Validate(_ pluginrpc.ValidateRequest, resp *pluginrpc.ValidateResponse) error {
	resp.OK = true
	return nil
}

func (minimalPlugin) Health(_ pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	resp.OK = true
	resp.State = "ready"
	return nil
}

func (minimalPlugin) CallHook(req pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
	log.Printf("minimal: event=%s plugin=%s/%s", req.EventName, req.PluginID, req.Version)
	resp.Status = "ok"
	return nil
}

func (minimalPlugin) MigrateConfig(req pluginrpc.MigrateConfigRequest, resp *pluginrpc.MigrateConfigResponse) error {
	resp.OK = true
	resp.MigratedConfig = req.CurrentConfig
	return nil
}

func main() {
	rpcplugin.Serve(minimalPlugin{})
}
