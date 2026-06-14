package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/jahrulnr/gosite/internal/service/docker"
)

// DockerHandler serves docker container endpoints.
type DockerHandler struct {
	svc *docker.Service
}

// NewDockerHandler returns a docker handler.
func NewDockerHandler(svc *docker.Service) *DockerHandler {
	return &DockerHandler{svc: svc}
}

// List handles GET /docker/containers.
func (h *DockerHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"containers": rows})
}

// Restart handles POST /docker/containers/{id}/restart.
func (h *DockerHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id, err := containerID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.Restart(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "container restarted"})
}

// Stop handles POST /docker/containers/{id}/stop.
func (h *DockerHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id, err := containerID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.Stop(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "container stopped"})
}

// Logs handles GET /docker/containers/{id}/logs.
func (h *DockerHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id, err := containerID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	tail := 200
	if raw := r.URL.Query().Get("tail"); raw != "" {
		if v, parseErr := strconv.Atoi(raw); parseErr == nil {
			tail = v
		}
	}
	logs, err := h.svc.Logs(r.Context(), id, tail)
	if err != nil {
		writeError(w, err)
		return
	}
	lines := []string{}
	if trimmed := strings.TrimSpace(logs); trimmed != "" {
		lines = strings.Split(trimmed, "\n")
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"lines": lines})
}
