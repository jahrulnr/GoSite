package handler

import (
	"net/http"
	"strings"

	"github.com/jahrulnr/gosite/internal/service/database"
)

// DatabaseHandler serves read-only SQLite viewer endpoints.
type DatabaseHandler struct {
	svc *database.Service
}

// NewDatabaseHandler returns a database viewer handler.
func NewDatabaseHandler(svc *database.Service) *DatabaseHandler {
	return &DatabaseHandler{svc: svc}
}

// ListTables handles GET /database/tables.
func (h *DatabaseHandler) ListTables(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.ListTables(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetTable handles GET /database/tables/{name}.
func (h *DatabaseHandler) GetTable(w http.ResponseWriter, r *http.Request) {
	name := tableNameFromPath(r)
	if name == "" {
		if v := r.PathValue("name"); v != "" {
			name = v
		}
	}
	limit := queryInt(r, "limit", 100)
	offset := queryInt(r, "offset", 0)

	result, err := h.svc.GetTable(r.Context(), name, limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func tableNameFromPath(r *http.Request) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if p == "tables" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
