// Package pluginrpc defines the Go-native gRPC contract for Tier 1 plugins.
//
// The contract is intentionally small and stable across GoSite releases:
//   - Validate:  dry-run + contract check at install time
//   - Health:    periodic liveness check while enabled
//   - CallHook:  dispatch host lifecycle events to the plugin
//   - MigrateConfig: optional configVersion migration during switch
//
// The same contract is exposed to the host in two ways:
//  1. As a net/rpc service surface that HashiCorp go-plugin uses by default
//     (no protobuf toolchain required).
//  2. As a generated gRPC service once the project adopts protobuf codegen.
package pluginrpc

// PluginManifestSnapshot is the host-supplied context for Validate and
// MigrateConfig. It mirrors the on-disk manifest fields the plugin needs
// to make compatibility decisions.
type PluginManifestSnapshot struct {
	PluginID         string `json:"plugin_id"`
	Version          string `json:"version"`
	Tier             int    `json:"tier"`
	APIVersion       string `json:"api_version"`
	MinGoSiteVersion string `json:"min_gosite_version"`
	RPCVersion       string `json:"rpc_version"`
	ConfigVersion    string `json:"config_version"`
}

// ValidateRequest is the payload of Plugin.Validate.
type ValidateRequest struct {
	Manifest PluginManifestSnapshot `json:"manifest"`
	DryRun   bool                   `json:"dry_run"`
}

// ValidateResponse carries the validation outcome.
type ValidateResponse struct {
	OK          bool     `json:"ok"`
	Errors      []string `json:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// HealthRequest is the payload of Plugin.Health.
type HealthRequest struct {
	PluginID string `json:"plugin_id"`
	Version  string `json:"version"`
}

// HealthResponse reports the plugin's current state.
type HealthResponse struct {
	OK    bool   `json:"ok"`
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

// CallHookRequest dispatches one host event to the plugin.
type CallHookRequest struct {
	EventName     string `json:"event_name"`
	PayloadJSON   string `json:"payload_json"`
	Strict        bool   `json:"strict"`
	PluginID      string `json:"plugin_id"`
	Version       string `json:"version"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

// CallHookResponse is the plugin's reply to a hook invocation.
type CallHookResponse struct {
	Status     string `json:"status"` // "ok" | "error"
	Error      string `json:"error,omitempty"`
	SideEffect string `json:"side_effect,omitempty"`
}

// MigrateConfigRequest carries the previous config and metadata for
// schema migration during a switch.
type MigrateConfigRequest struct {
	Current        PluginManifestSnapshot `json:"current"`
	Next           PluginManifestSnapshot `json:"next"`
	CurrentConfig  string                 `json:"current_config"`  // raw JSON or empty
	NextSchemaHint string                `json:"next_schema_hint,omitempty"`
}

// MigrateConfigResponse reports the migration outcome.
type MigrateConfigResponse struct {
	OK             bool   `json:"ok"`
	MigratedConfig string `json:"migrated_config,omitempty"`
	Error          string `json:"error,omitempty"`
}

// HandshakeConfig is the protocol negotiation used by HashiCorp go-plugin.
// Plugins import this value from the host package to declare support.
var HandshakeConfig = map[string][]string{
	"gosite_plugin_protocol": {"1"},
}

// HandshakeMagic is the magic cookie value paired with HandshakeConfig.
const HandshakeMagic = "gosite-plugin"

// PluginMap is the canonical map of plugin name -> implementation factory.
// The key MUST be a stable identifier (the manifest entrypoints.runtime
// command basename); the value is a factory returning the plugin gRPC
// server-side implementation.
var PluginMap = map[string]PluginServerFactory{}

// PluginServerFactory builds a server-side plugin implementation. Plugins
// register their factory once at init() time and the host invokes it when
// a matching tier-1 plugin is launched.
type PluginServerFactory func() (Plugin, error)

// Plugin is the contract a tier-1 plugin must implement. Implementations
// are reachable from the host over gRPC via HashiCorp go-plugin.
type Plugin interface {
	Validate(req ValidateRequest, resp *ValidateResponse) error
	Health(req HealthRequest, resp *HealthResponse) error
	CallHook(req CallHookRequest, resp *CallHookResponse) error
	MigrateConfig(req MigrateConfigRequest, resp *MigrateConfigResponse) error
}

// Serve is invoked by the plugin binary's main(). It is provided here so
// plugins can share a single import path: gosite/pkg/pluginrpc.Serve.
func Serve(name string, impl Plugin) {
	if _, exists := PluginMap[name]; exists {
		panic("pluginrpc: duplicate plugin registration for " + name)
	}
	PluginMap[name] = func() (Plugin, error) { return impl, nil }
}
