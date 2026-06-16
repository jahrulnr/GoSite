package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/service/ssl"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// SSLHandler serves website SSL endpoints.
type SSLHandler struct {
	svc    *ssl.Service
	worker *job.Worker
}

// NewSSLHandler returns an SSL handler.
func NewSSLHandler(svc *ssl.Service, worker *job.Worker) *SSLHandler {
	return &SSLHandler{svc: svc, worker: worker}
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
		writeError(w, apperror.New(apperror.CodeInvalidInput, "job_id required"))
		return
	}
	jobID, err := parseID(jobIDStr)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.worker.StreamSSE(r.Context(), w, jobID); err != nil {
		if appErr := apperror.From(err); appErr != nil && appErr.Code == apperror.CodeJobFailed {
			return
		}
		writeError(w, err)
	}
}

// RegisterSSLRoutes registers SSL routes on mux.
func RegisterSSLRoutes(mux *http.ServeMux, h *SSLHandler) {
	mux.HandleFunc("GET /api/v1/websites/{id}/ssl", h.GetStatus)
	mux.HandleFunc("PUT /api/v1/websites/{id}/ssl/manual", h.UpdateManual)
	mux.HandleFunc("POST /api/v1/websites/{id}/ssl/certbot", h.StartCertbot)
	mux.HandleFunc("GET /api/v1/websites/{id}/ssl/certbot/stream", h.CertbotStream)
}
