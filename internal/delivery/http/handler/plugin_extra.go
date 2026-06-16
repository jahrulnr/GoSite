package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	pluginsvc "github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// ConfigHandler serves plugin configuration and secret endpoints.
type ConfigHandler struct {
	svc *pluginsvc.ConfigService
}

// NewConfigHandler returns a config handler bound to the supplied service.
func NewConfigHandler(svc *pluginsvc.ConfigService) *ConfigHandler {
	return &ConfigHandler{svc: svc}
}

// Get handles GET /plugins/{vendor}/{name}/versions/{version}/config.
func (h *ConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
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
	view, err := h.svc.Get(r.Context(), pluginID, version)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// Put handles PUT /plugins/{vendor}/{name}/versions/{version}/config.
func (h *ConfigHandler) Put(w http.ResponseWriter, r *http.Request) {
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
	var input pluginsvc.ConfigInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(input.Version) == "" {
		input.Version = version
	}
	view, err := h.svc.Put(r.Context(), pluginID, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// KeyringHandler manages the trusted vendor keyring.
type KeyringHandler struct {
	path string
}

// NewKeyringHandler returns a keyring handler backed by the given JSON
// file path. The path is supplied by config and matches Service.keyringPath.
func NewKeyringHandler(path string) *KeyringHandler {
	return &KeyringHandler{path: path}
}

// List handles GET /plugins/keyring.
func (h *KeyringHandler) List(w http.ResponseWriter, r *http.Request) {
	keys, err := pluginsvc.LoadKeyring(r.Context(), h.path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"keys": keys})
}

// Add handles POST /plugins/keyring.
func (h *KeyringHandler) Add(w http.ResponseWriter, r *http.Request) {
	var key pluginsvc.TrustedKey
	if err := decodeJSON(r, &key); err != nil {
		writeError(w, err)
		return
	}
	if err := pluginsvc.AddKeyringEntry(r.Context(), h.path, key); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// Revoke handles DELETE /plugins/keyring/{keyId}.
func (h *KeyringHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	vendor := strings.TrimSpace(r.URL.Query().Get("vendor"))
	keyID := strings.TrimSpace(r.URL.Query().Get("keyId"))
	if vendor == "" || keyID == "" {
		writeError(w, apperror.New(apperror.CodeInvalidInput, "vendor and keyId required"))
		return
	}
	if err := pluginsvc.RevokeKeyringEntry(r.Context(), h.path, vendor, keyID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var _ = json.Marshal
