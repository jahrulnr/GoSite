package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/service/system"
)

// SystemHandler serves host monitoring endpoints.
type SystemHandler struct {
	svc *system.Service
}

// NewSystemHandler returns a system metrics handler.
func NewSystemHandler(svc *system.Service) *SystemHandler {
	return &SystemHandler{svc: svc}
}

// Info handles GET /system/info.
func (h *SystemHandler) Info(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.Info(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Network handles GET /system/network.
func (h *SystemHandler) Network(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.Network(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// DiskIO handles GET /system/disk-io.
func (h *SystemHandler) DiskIO(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.DiskIO(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// NginxTraffic handles GET /system/nginx-traffic.
func (h *SystemHandler) NginxTraffic(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.NginxTraffic(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
