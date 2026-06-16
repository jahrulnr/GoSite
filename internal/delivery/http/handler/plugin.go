package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	pluginsvc "github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/jahrulnr/gosite/internal/service/plugin/remote"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// PluginHandler serves plugin installer and lifecycle endpoints.
type PluginHandler struct {
	svc       *pluginsvc.Service
	remote    *remote.Service
	remoteCfg remote.Config
}

// NewPluginHandler returns a plugin handler.
func NewPluginHandler(svc *pluginsvc.Service, remoteSvc *remote.Service, remoteCfg remote.Config) *PluginHandler {
	return &PluginHandler{svc: svc, remote: remoteSvc, remoteCfg: remoteCfg}
}

type installLogStepJSON struct {
	Step         string `json:"step"`
	At           string `json:"at"`
	Status       string `json:"status"`
	FailureClass string `json:"failure_class,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type pluginJSON struct {
	ID               int64          `json:"id"`
	PluginID         string         `json:"plugin_id"`
	Version          string         `json:"version"`
	Name             string         `json:"name"`
	Tier             int            `json:"tier"`
	APIVersion       string         `json:"api_version"`
	MinGoSiteVersion string         `json:"min_gosite_version"`
	RPCVersion       string         `json:"rpc_version,omitempty"`
	ConfigVersion    string         `json:"config_version,omitempty"`
	ArtifactDigest   string         `json:"artifact_digest"`
	State            string         `json:"state"`
	FailureClass     string         `json:"failure_class,omitempty"`
	FailureMessage   string         `json:"failure_message,omitempty"`
	FailureAt        *string        `json:"failure_at,omitempty"`
	Manifest         map[string]any `json:"manifest"`
	Capabilities     map[string]any `json:"capabilities"`
	UI               map[string]any `json:"ui"`
	SourceType       string         `json:"source_type,omitempty"`
	SourceRef        string         `json:"source_ref,omitempty"`
	ResolvedURL      string         `json:"resolved_url,omitempty"`
	InstallPath      string         `json:"install_path,omitempty"`
	SourceCommit     string         `json:"source_commit,omitempty"`
	PermissionsAckAt *string        `json:"permissions_ack_at,omitempty"`
	InstallLog       []installLogStepJSON `json:"install_log,omitempty"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

// List handles GET /plugins.
func (h *PluginHandler) List(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]pluginJSON, 0, len(plugins))
	for _, item := range plugins {
		out = append(out, pluginDTO(item))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"plugins": out})
}

// Install handles POST /plugins/install.
func (h *PluginHandler) Install(w http.ResponseWriter, r *http.Request) {
	input, err := h.readInstallInput(r)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin, err := h.svc.Install(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"plugin": pluginDTO(plugin)})
}

// Resolve handles POST /plugins/install/resolve.
func (h *PluginHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	if h.remote == nil {
		writeError(w, apperror.New(apperror.CodeInvalidInput, remote.FailureRemoteInstallDisabled))
		return
	}
	var body struct {
		Source remote.Source `json:"source"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	preview, err := h.remote.Resolve(r.Context(), body.Source)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"preview": preview})
}

// InstallSettings handles GET /plugins/install/settings.
func (h *PluginHandler) InstallSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"remote_install_enabled": h.remoteCfg.Enabled,
		"trust_mode":             h.remoteCfg.TrustMode,
		"allowed_hosts":          h.remoteCfg.AllowedHosts,
	})
}

// Enable handles POST /plugins/{vendor}/{name}/enable.
func (h *PluginHandler) Enable(w http.ResponseWriter, r *http.Request) {
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Version string `json:"version"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, err)
			return
		}
	}
	plugin, err := h.svc.Enable(r.Context(), pluginID, body.Version)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"plugin": pluginDTO(plugin)})
}

// Disable handles POST /plugins/{vendor}/{name}/disable.
func (h *PluginHandler) Disable(w http.ResponseWriter, r *http.Request) {
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	plugin, err := h.svc.Disable(r.Context(), pluginID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"plugin": pluginDTO(plugin)})
}

// Switch handles POST /plugins/{vendor}/{name}/switch.
func (h *PluginHandler) Switch(w http.ResponseWriter, r *http.Request) {
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(body.Version) == "" {
		writeError(w, apperror.New(apperror.CodeInvalidInput, "version required"))
		return
	}
	plugin, err := h.svc.SwitchEnabledVersion(r.Context(), pluginID, body.Version)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"plugin": pluginDTO(plugin)})
}

// Uninstall handles DELETE /plugins/{vendor}/{name}/versions/{version}.
func (h *PluginHandler) Uninstall(w http.ResponseWriter, r *http.Request) {
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	version := pluginVersionFromPath(r)
	if version == "" {
		writeError(w, apperror.New(apperror.CodeInvalidInput, "version required"))
		return
	}
	if strings.EqualFold(r.URL.Query().Get("purge"), "true") {
		if err := h.svc.Purge(r.Context(), pluginID, version); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "purged"})
		return
	}
	plugin, err := h.svc.Uninstall(r.Context(), pluginID, version)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"plugin": pluginDTO(plugin)})
}

func (h *PluginHandler) readInstallInput(r *http.Request) (pluginsvc.InstallInput, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			return pluginsvc.InstallInput{}, apperror.Wrap(apperror.CodeInvalidInput, "invalid multipart body", err)
		}
		file, header, err := r.FormFile("artifact")
		if err != nil {
			return pluginsvc.InstallInput{}, apperror.Wrap(apperror.CodeInvalidInput, "artifact file required", err)
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, 64<<20))
		if err != nil {
			return pluginsvc.InstallInput{}, apperror.Wrap(apperror.CodeInvalidInput, "read artifact failed", err)
		}
		return pluginsvc.InstallInput{
			Name:           header.Filename,
			Content:        data,
			ExpectedSHA256: r.FormValue("sha256"),
		}, nil
	}

	var body struct {
		Name           string          `json:"name"`
		SHA256         string          `json:"sha256"`
		Content        json.RawMessage `json:"content"`
		Source         *remote.Source  `json:"source"`
		PermissionsAck bool            `json:"permissions_ack"`
		ResolveToken   string          `json:"resolveToken"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return pluginsvc.InstallInput{}, err
	}
	if body.Source != nil {
		if h.remote == nil {
			return pluginsvc.InstallInput{}, apperror.New(apperror.CodeInvalidInput, remote.FailureRemoteInstallDisabled)
		}
		source := *body.Source
		if strings.TrimSpace(body.ResolveToken) != "" {
			source.ResolveToken = strings.TrimSpace(body.ResolveToken)
		}
		plan, data, err := h.remote.ResolveAndFetch(r.Context(), source, source.ResolveToken)
		if err != nil {
			return pluginsvc.InstallInput{}, err
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			name = "plugin.zip"
		}
		return pluginsvc.InstallInput{
			Name:           name,
			Content:        data,
			ExpectedSHA256: plan.SHA256,
			PermissionsAck: body.PermissionsAck,
			Provenance: &pluginsvc.InstallProvenance{
				SourceType:       plan.SourceType,
				SourceRef:        plan.SourceRef,
				ResolvedURL:      plan.URL,
				ResolvedDigest:   plan.ResolvedDigest,
				SourceCommit:     plan.SourceCommit,
				SourceRepository: plan.SourceRepository,
				InstallPath:      plan.InstallPath,
			},
		}, nil
	}

	var content string
	if err := json.Unmarshal(body.Content, &content); err != nil {
		return pluginsvc.InstallInput{}, apperror.New(apperror.CodeInvalidInput, "content must be a json string")
	}
	return pluginsvc.InstallInput{
		Name:           body.Name,
		Content:        []byte(content),
		ExpectedSHA256: body.SHA256,
	}, nil
}

func pluginDTO(plugin sqlite.PluginVersion) pluginJSON {
	manifest := decodeJSONObject(plugin.ManifestJSON)
	capabilities := decodeJSONObject(plugin.CapabilitiesJSON)
	ui := decodeJSONObject(plugin.UIJSON)
	out := pluginJSON{
		ID:               plugin.ID,
		PluginID:         plugin.PluginID,
		Version:          plugin.Version,
		Name:             plugin.Name,
		Tier:             plugin.Tier,
		APIVersion:       plugin.APIVersion,
		MinGoSiteVersion: plugin.MinGoSiteVersion,
		RPCVersion:       plugin.RPCVersion,
		ConfigVersion:    plugin.ConfigVersion,
		ArtifactDigest:   plugin.ArtifactDigest,
		State:            plugin.State,
		FailureClass:     plugin.FailureClass,
		FailureMessage:   plugin.FailureMessage,
		Manifest:         manifest,
		Capabilities:     capabilities,
		UI:               ui,
		SourceType:       plugin.SourceType,
		SourceRef:        plugin.SourceRef,
		ResolvedURL:      plugin.ResolvedURL,
		InstallPath:      plugin.InstallPath,
		SourceCommit:     plugin.SourceCommit,
		CreatedAt:        plugin.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:        plugin.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if plugin.FailureAt != nil {
		ts := plugin.FailureAt.UTC().Format("2006-01-02T15:04:05Z")
		out.FailureAt = &ts
	}
	if plugin.PermissionsAckAt != nil {
		ts := plugin.PermissionsAckAt.UTC().Format("2006-01-02T15:04:05Z")
		out.PermissionsAckAt = &ts
	}
	out.InstallLog = decodeInstallLog(plugin.InstallLog)
	return out
}

func decodeInstallLog(raw string) []installLogStepJSON {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return nil
	}
	var steps []installLogStepJSON
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return nil
	}
	return steps
}

func decodeJSONObject(raw string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func pluginIDFromPath(r *http.Request) (string, error) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, part := range parts {
		if part == "plugins" && i+2 < len(parts) {
			vendor := strings.TrimSpace(parts[i+1])
			name := strings.TrimSpace(parts[i+2])
			if vendor != "" && name != "" {
				return vendor + "/" + name, nil
			}
		}
	}
	if id := strings.TrimSpace(r.URL.Query().Get("id")); id != "" {
		return id, nil
	}
	return "", apperror.New(apperror.CodeInvalidInput, "plugin id required")
}

func pluginVersionFromPath(r *http.Request) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, part := range parts {
		if part == "versions" && i+1 < len(parts) {
			return strings.TrimSpace(parts[i+1])
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("version"))
}
