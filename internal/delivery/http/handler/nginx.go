package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/infra/nginx"
)

// NginxHandler serves global nginx configuration endpoints.
type NginxHandler struct {
	nginx *nginx.Service
}

// NewNginxHandler returns an nginx handler.
func NewNginxHandler(ngx *nginx.Service) *NginxHandler {
	return &NginxHandler{nginx: ngx}
}

// GetDefault handles GET /nginx/default.
func (h *NginxHandler) GetDefault(w http.ResponseWriter, r *http.Request) {
	content, err := h.nginx.ReadDefaultConfig()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"config": content})
}

// UpdateDefault handles PUT /nginx/default.
func (h *NginxHandler) UpdateDefault(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Config string `json:"config"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.nginx.UpdateDefaultConfig(r.Context(), body.Config); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Update successfully"})
}

// GetGlobal handles GET /nginx/global.
func (h *NginxHandler) GetGlobal(w http.ResponseWriter, r *http.Request) {
	content, err := h.nginx.ReadGlobalConfig()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"config": content})
}

// UpdateGlobal handles PUT /nginx/global.
func (h *NginxHandler) UpdateGlobal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
		Config  string `json:"config"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	content := body.Content
	if content == "" {
		content = body.Config
	}
	if err := h.nginx.UpdateGlobalConfig(r.Context(), content); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Nginx configuration updated successfully."})
}

// Test handles POST /nginx/test.
func (h *NginxHandler) Test(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Config string `json:"config"`
		Scope  string `json:"scope"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	var err error
	switch body.Scope {
	case "default":
		err = h.nginx.TestDefaultConfig(r.Context(), body.Config)
	default:
		err = h.nginx.TestRawConfig(r.Context(), body.Config)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Reload handles POST /nginx/reload.
func (h *NginxHandler) Reload(w http.ResponseWriter, r *http.Request) {
	if err := h.nginx.Reload(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "nginx reloaded"})
}

// RegisterNginxRoutes registers nginx admin routes.
func RegisterNginxRoutes(mux *http.ServeMux, h *NginxHandler) {
	mux.HandleFunc("GET /api/v1/nginx/default", h.GetDefault)
	mux.HandleFunc("PUT /api/v1/nginx/default", h.UpdateDefault)
	mux.HandleFunc("GET /api/v1/nginx/global", h.GetGlobal)
	mux.HandleFunc("PUT /api/v1/nginx/global", h.UpdateGlobal)
	mux.HandleFunc("POST /api/v1/nginx/test", h.Test)
}
