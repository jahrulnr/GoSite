package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/service/cron"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// CronHandler serves cron job endpoints.
type CronHandler struct {
	svc    *cron.Service
	worker *job.Worker
}

// NewCronHandler returns a cron handler.
func NewCronHandler(svc *cron.Service, worker *job.Worker) *CronHandler {
	return &CronHandler{svc: svc, worker: worker}
}

type cronJobJSON struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Payload    string  `json:"payload"`
	RunEvery   string  `json:"run_every"`
	ExecutedAt *string `json:"executed_at,omitempty"`
}

// List handles GET /cronjobs.
func (h *CronHandler) List(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.svc.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]cronJobJSON, 0, len(jobs))
	for _, job := range jobs {
		item := cronJobJSON{
			ID:       job.ID,
			Name:     job.Name,
			Payload:  job.Payload,
			RunEvery: job.RunEvery,
		}
		if job.ExecutedAt != nil {
			ts := job.ExecutedAt.UTC().Format("2006-01-02T15:04:05Z")
			item.ExecutedAt = &ts
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"cronjobs": out})
}

// Create handles POST /cronjobs.
func (h *CronHandler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Payload  string `json:"payload"`
		RunEvery string `json:"run_every"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	job, err := h.svc.Create(r.Context(), cron.CreateInput{
		Name:     body.Name,
		Payload:  body.Payload,
		RunEvery: body.RunEvery,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cronJobJSON{
		ID:       job.ID,
		Name:     job.Name,
		Payload:  job.Payload,
		RunEvery: job.RunEvery,
	})
}

// Update handles PUT /cronjobs/{id}.
func (h *CronHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := resourceID(r, "cronjobs")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name     string `json:"name"`
		Payload  string `json:"payload"`
		RunEvery string `json:"run_every"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	job, err := h.svc.Update(r.Context(), id, cron.CreateInput{
		Name:     body.Name,
		Payload:  body.Payload,
		RunEvery: body.RunEvery,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cronJobJSON{
		ID:       job.ID,
		Name:     job.Name,
		Payload:  job.Payload,
		RunEvery: job.RunEvery,
	})
}

// Delete handles DELETE /cronjobs/{id}.
func (h *CronHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := resourceID(r, "cronjobs")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

// Run handles POST /cronjobs/{id}/run.
func (h *CronHandler) Run(w http.ResponseWriter, r *http.Request) {
	id, err := resourceID(r, "cronjobs")
	if err != nil {
		writeError(w, err)
		return
	}
	run, err := h.svc.RunManual(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":  run.ID,
		"message": "Waiting task on queue",
	})
}

// RunStream handles GET /cronjobs/{id}/run/stream.
func (h *CronHandler) RunStream(w http.ResponseWriter, r *http.Request) {
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
