package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/service/uimeta"
)

// UIMetaHandler serves runtime UI metadata and capabilities.
type UIMetaHandler struct {
	svc *uimeta.Service
}

func NewUIMetaHandler(svc *uimeta.Service) *UIMetaHandler {
	return &UIMetaHandler{svc: svc}
}

// Get handles GET /ui/meta.
func (h *UIMetaHandler) Get(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Get())
}
