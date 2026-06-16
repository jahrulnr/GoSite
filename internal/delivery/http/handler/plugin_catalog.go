package handler

import (
	"errors"
	"net/http"

	"github.com/jahrulnr/gosite/internal/service/plugin/catalog"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// CatalogList handles GET /plugins/catalog.
func (h *PluginHandler) CatalogList(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"entries": []any{}})
		return
	}
	entries, err := h.catalog.List(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"entries": entries})
}

// CatalogGet handles GET /plugins/catalog/{vendor}/{name}.
func (h *PluginHandler) CatalogGet(w http.ResponseWriter, r *http.Request) {
	if h.catalog == nil {
		writeError(w, apperror.New(apperror.CodeNotFound, "catalog entry not found"))
		return
	}
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	entry, err := h.catalog.Get(r.Context(), pluginID)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			writeError(w, apperror.New(apperror.CodeNotFound, "catalog entry not found"))
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"entry": entry})
}
