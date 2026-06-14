package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jahrulnr/gosite/internal/service/ssl"
)

// SSLHandler serves website SSL endpoints.
type SSLHandler struct {
	svc *ssl.Service
}

// NewSSLHandler returns an SSL handler.
func NewSSLHandler(svc *ssl.Service) *SSLHandler {
	return &SSLHandler{svc: svc}
}

// GetStatus handles GET /websites/{id}/ssl.
func (h *SSLHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	status, err := h.svc.GetStatus(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// UpdateManual handles PUT /websites/{id}/ssl/manual.
func (h *SSLHandler) UpdateManual(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Public  string `json:"public"`
		Private string `json:"private"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.UpdateManual(r.Context(), id, ssl.ManualInput{
		Public:  body.Public,
		Private: body.Private,
	}); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Update SSL successfully"})
}

// StartCertbot handles POST /websites/{id}/ssl/certbot.
func (h *SSLHandler) StartCertbot(w http.ResponseWriter, r *http.Request) {
	id, err := requestID(r)
	if err != nil {
		writeError(w, err)
		return
	}
	job, err := h.svc.EnqueueCertbot(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":  job.ID,
		"message": "Waiting task on queue",
	})
}

// CertbotStream handles GET /websites/{id}/ssl/certbot/stream.
func (h *SSLHandler) CertbotStream(w http.ResponseWriter, r *http.Request) {
	jobIDStr := r.URL.Query().Get("job_id")
	if jobIDStr == "" {
		writeError(w, fmt.Errorf("job_id required"))
		return
	}
	jobID, err := parseID(jobIDStr)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, fmt.Errorf("streaming unsupported"))
		return
	}

	job, err := h.svc.GetCertbotJob(r.Context(), jobID)
	if err != nil {
		writeError(w, err)
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", job.Output)
	flusher.Flush()

	if job.Status == "pending" {
		fmt.Fprintf(w, "data: Waiting task on queue\n\n")
		flusher.Flush()
	}

	for i := 0; i < 3; i++ {
		time.Sleep(10 * time.Millisecond)
		job, err = h.svc.GetCertbotJob(r.Context(), jobID)
		if err != nil {
			break
		}
		fmt.Fprintf(w, "data: status=%s\n\n", job.Status)
		flusher.Flush()
	}
}

// RegisterSSLRoutes registers SSL routes on mux.
func RegisterSSLRoutes(mux *http.ServeMux, h *SSLHandler) {
	mux.HandleFunc("GET /api/v1/websites/{id}/ssl", h.GetStatus)
	mux.HandleFunc("PUT /api/v1/websites/{id}/ssl/manual", h.UpdateManual)
	mux.HandleFunc("POST /api/v1/websites/{id}/ssl/certbot", h.StartCertbot)
	mux.HandleFunc("GET /api/v1/websites/{id}/ssl/certbot/stream", h.CertbotStream)
}
