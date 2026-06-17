package main

import (
	"github.com/jahrulnr/gosite/pkg/pluginrpc"
	"github.com/jahrulnr/gosite/plugins/_templates/_shared/rpcplugin"
)

type templatePlugin struct{}

func (templatePlugin) Validate(_ pluginrpc.ValidateRequest, resp *pluginrpc.ValidateResponse) error {
	resp.OK = true
	return nil
}

func (templatePlugin) Health(_ pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	resp.OK = true
	resp.State = "ready"
	return nil
}

func (templatePlugin) CallHook(_ pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
	resp.Status = "ok"
	return nil
}

func (templatePlugin) MigrateConfig(req pluginrpc.MigrateConfigRequest, resp *pluginrpc.MigrateConfigResponse) error {
	resp.OK = true
	resp.MigratedConfig = req.CurrentConfig
	return nil
}

func main() {
	rpcplugin.Serve(templatePlugin{})
}
