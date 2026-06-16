package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/website"
)

// WebsiteHandler serves website CRUD endpoints.
type WebsiteHandler struct {
	svc *website.Service
}

// NewWebsiteHandler returns a website handler.
func NewWebsiteHandler(svc *website.Service) *WebsiteHandler {
	return &WebsiteHandler{svc: svc}
}

type websiteJSON struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Upstream string `json:"upstream,omitempty"`
	SSL      bool   `json:"ssl"`
	Active   bool   `json:"active"`
	Config   string `json:"config,omitempty"`
}

func mapWebsite(s sqlite.Website) websiteJSON {
	return websiteJSON{
		ID:       s.ID,
		Name:     s.Name,
		Domain:   s.Domain,
		Path:     s.Path,
		Type:     s.Type,
		Upstream: s.Upstream,
		SSL:      s.SSL,
		Active:   s.Active,
		Config:   s.Config,
	}
}

// List handles GET /websites.
func (h *WebsiteHandler) List(w http.ResponseWriter, r *http.Request) {
	sites, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]websiteJSON, 0, len(sites))
	for _, s := range sites {
		out = append(out, mapWebsite(s))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"websites": out})
}

// Get handles GET /websites/{id}.
func (h *WebsiteHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	site, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapWebsite(site))
}

// Create handles POST /websites.
func (h *WebsiteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Domain   string `json:"domain"`
		Path     string `json:"path"`
		Type     string `json:"type"`
		Upstream string `json:"upstream"`
		Active   bool   `json:"active"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	site, err := h.svc.Create(r.Context(), website.CreateInput{
		Name:     body.Name,
		Domain:   body.Domain,
		Path:     body.Path,
		Type:     body.Type,
		Upstream: body.Upstream,
		Active:   body.Active,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapWebsite(site))
}

// Update handles PUT /websites/{id}.
func (h *WebsiteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name     string `json:"name"`
		Domain   string `json:"domain"`
		Path     string `json:"path"`
		Type     string `json:"type"`
		Upstream string `json:"upstream"`
		Active   bool   `json:"active"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	site, err := h.svc.Update(r.Context(), id, website.UpdateInput{
		Name:     body.Name,
		Domain:   body.Domain,
		Path:     body.Path,
		Type:     body.Type,
		Upstream: body.Upstream,
		Active:   body.Active,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapWebsite(site))
}

// Delete handles DELETE /websites/{id}?clean=true|false.
func (h *WebsiteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	clean := r.URL.Query().Get("clean") == "true"
	if err := h.svc.Delete(r.Context(), id, clean); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": website.DeleteMessage})
}

// Toggle handles PATCH /websites/{id}/toggle.
func (h *WebsiteHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	site, err := h.svc.Toggle(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      site.ID,
		"active":  site.Active,
		"message": website.FormatToggleMessage(site.Active),
	})
}

// Validate handles POST /websites/validate.
func (h *WebsiteHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain   string `json:"domain"`
		Path     string `json:"path"`
		Type     string `json:"type"`
		Upstream string `json:"upstream"`
		Active   bool   `json:"active"`
		ID       int64  `json:"id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	result := h.svc.Validate(r.Context(), website.ValidateInput{
		Domain:    body.Domain,
		Path:      body.Path,
		Type:      body.Type,
		Upstream:  body.Upstream,
		Active:    body.Active,
		ExcludeID: body.ID,
	})
	writeJSON(w, http.StatusOK, result)
}

// TestNginxConfig handles POST /websites/{id}/nginx-config/test.
func (h *WebsiteHandler) TestNginxConfig(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Config string `json:"config"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.TestNginxConfig(r.Context(), id, body.Config); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// UpdateNginxConfig handles PUT /websites/{id}/nginx-config.
func (h *WebsiteHandler) UpdateNginxConfig(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Config string `json:"config"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.UpdateNginxConfig(r.Context(), id, body.Config); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Update successfully"})
}

// GetNginxConfig handles GET /websites/{id}/nginx-config.
func (h *WebsiteHandler) GetNginxConfig(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	config, err := h.svc.GetNginxConfig(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"config": config})
}

// RegisterWebsiteRoutes registers website routes on mux.
func RegisterWebsiteRoutes(mux *http.ServeMux, h *WebsiteHandler) {
	mux.HandleFunc("GET /api/v1/websites", h.List)
	mux.HandleFunc("POST /api/v1/websites", h.Create)
	mux.HandleFunc("POST /api/v1/websites/validate", h.Validate)
	mux.HandleFunc("GET /api/v1/websites/{id}", h.Get)
	mux.HandleFunc("PUT /api/v1/websites/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/websites/{id}", h.Delete)
	mux.HandleFunc("PATCH /api/v1/websites/{id}/toggle", h.Toggle)
	mux.HandleFunc("GET /api/v1/websites/{id}/nginx-config", h.GetNginxConfig)
	mux.HandleFunc("PUT /api/v1/websites/{id}/nginx-config", h.UpdateNginxConfig)
	mux.HandleFunc("POST /api/v1/websites/{id}/nginx-config/test", h.TestNginxConfig)
}
