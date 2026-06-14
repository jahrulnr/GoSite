package handler

import (
	"net/http"
	"strconv"

	"github.com/jahrulnr/gosite/internal/service/logs"
)

// LogsHandler serves log viewer endpoints.
type LogsHandler struct {
	svc *logs.Service
}

// NewLogsHandler returns a logs handler.
func NewLogsHandler(svc *logs.Service) *LogsHandler {
	return &LogsHandler{svc: svc}
}

// ListSites handles GET /logs/sites.
func (h *LogsHandler) ListSites(w http.ResponseWriter, r *http.Request) {
	sites, err := h.svc.ListSites(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sites": sites})
}

// Tail handles GET /logs.
func (h *LogsHandler) Tail(w http.ResponseWriter, r *http.Request) {
	tail := queryInt(r, "tail", 1000)
	result, err := h.svc.Tail(r.Context(), logs.TailInput{
		Domain: r.URL.Query().Get("domain"),
		Type:   r.URL.Query().Get("type"),
		Tail:   tail,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}
