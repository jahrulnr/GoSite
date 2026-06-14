package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/service/mount"
)

// MountHandler serves fstab mount endpoints.
type MountHandler struct {
	svc *mount.Service
}

// NewMountHandler returns a mount handler.
func NewMountHandler(svc *mount.Service) *MountHandler {
	return &MountHandler{svc: svc}
}

// List handles GET /mounts.
func (h *MountHandler) List(w http.ResponseWriter, r *http.Request) {
	entries, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"mounts": entries})
}

// Create handles POST /mounts.
func (h *MountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body mount.Entry
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.Add(r.Context(), body); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": "mount added"})
}

// Update handles PUT /mounts.
func (h *MountHandler) Update(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldDevice string      `json:"old_device"`
		OldDir    string      `json:"old_dir"`
		Entry     mount.Entry `json:"entry"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	entry := body.Entry
	if err := h.svc.Update(r.Context(), body.OldDevice, body.OldDir, entry); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "mount updated"})
}

// Delete handles DELETE /mounts.
func (h *MountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	device := r.URL.Query().Get("device")
	dir := r.URL.Query().Get("dir")
	if err := h.svc.Delete(r.Context(), device, dir); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "mount deleted"})
}

// Enable handles POST /mounts/enable.
func (h *MountHandler) Enable(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Device string `json:"device"`
		Dir    string `json:"dir"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.Enable(r.Context(), body.Device, body.Dir); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "mount enabled"})
}
