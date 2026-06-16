// Production-shaped Tier 1 template: hooks, config migration, logging sink.
//
// Build: make build
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/jahrulnr/gosite/pkg/pluginrpc"
	"github.com/jahrulnr/gosite/plugins/_templates/_shared/rpcplugin"
)

type fullPlugin struct{}

func (fullPlugin) Validate(req pluginrpc.ValidateRequest, resp *pluginrpc.ValidateResponse) error {
	if req.Manifest.Tier != 1 {
		resp.OK = false
		resp.Errors = []string{"tier must be 1"}
		return nil
	}
	if req.Manifest.RPCVersion != "1" {
		resp.OK = false
		resp.Errors = []string{"unsupported rpcVersion"}
		return nil
	}
	resp.OK = true
	return nil
}

func (fullPlugin) Health(_ pluginrpc.HealthRequest, resp *pluginrpc.HealthResponse) error {
	resp.OK = true
	resp.State = "ready"
	return nil
}

func (fullPlugin) CallHook(req pluginrpc.CallHookRequest, resp *pluginrpc.CallHookResponse) error {
	switch req.EventName {
	case "logging.on_event":
		log.Printf("sink: %s", truncate(req.PayloadJSON, 512))
	case "nginx.before_reload", "site.before_create", "ssl.before_issue",
		"job.before_run", "cron.before_trigger", "container.before_action",
		"site.config_changed":
		if err := checkStrictPayload(req.PayloadJSON); err != nil {
			resp.Status = "error"
			resp.Error = err.Error()
			return nil
		}
		log.Printf("strict hook ok: %s", req.EventName)
	default:
		log.Printf("lenient hook: %s", req.EventName)
	}
	resp.Status = "ok"
	return nil
}

func (fullPlugin) MigrateConfig(req pluginrpc.MigrateConfigRequest, resp *pluginrpc.MigrateConfigResponse) error {
	from := req.Current.ConfigVersion
	to := req.Next.ConfigVersion
	if from == to {
		resp.OK = true
		resp.MigratedConfig = req.CurrentConfig
		return nil
	}
	var cfg map[string]any
	if strings.TrimSpace(req.CurrentConfig) != "" {
		if err := json.Unmarshal([]byte(req.CurrentConfig), &cfg); err != nil {
			resp.OK = false
			resp.Error = "invalid current config json"
			return nil
		}
	} else {
		cfg = map[string]any{}
	}
	// Example: v1 → v2 renames webhookUrl → endpoint
	if from == "1" && to == "2" {
		if v, ok := cfg["webhookUrl"]; ok {
			cfg["endpoint"] = v
			delete(cfg, "webhookUrl")
		}
		cfg["schemaVersion"] = 2
	}
	out, err := json.Marshal(cfg)
	if err != nil {
		resp.OK = false
		resp.Error = err.Error()
		return nil
	}
	resp.OK = true
	resp.MigratedConfig = string(out)
	return nil
}

func checkStrictPayload(raw string) error {
	if strings.TrimSpace(raw) == "" || raw == "null" {
		return fmt.Errorf("empty payload")
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return fmt.Errorf("payload must be json object")
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func main() {
	rpcplugin.Serve(fullPlugin{})
}
