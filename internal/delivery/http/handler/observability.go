package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jahrulnr/gosite/internal/observability/grafanalite"
	"github.com/jahrulnr/gosite/internal/observability/splunklite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// ObservabilityHandler serves Splunk Lite and Grafana Lite endpoints.
type ObservabilityHandler struct {
	splunk   *splunklite.Service
	meta     *splunklite.MetaService
	ingestor *splunklite.LogIngestor
	grafana  *grafanalite.Service
}

// NewObservabilityHandler returns an observability handler.
func NewObservabilityHandler(splunk *splunklite.Service, meta *splunklite.MetaService, ingestor *splunklite.LogIngestor, grafana *grafanalite.Service) *ObservabilityHandler {
	return &ObservabilityHandler{splunk: splunk, meta: meta, ingestor: ingestor, grafana: grafana}
}

// QueryMeta handles GET /query/meta.
func (h *ObservabilityHandler) QueryMeta(w http.ResponseWriter, r *http.Request) {
	meta, err := h.meta.Meta(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

// Query handles POST /query.
func (h *ObservabilityHandler) Query(w http.ResponseWriter, r *http.Request) {
	var req splunklite.QueryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if h.ingestor != nil {
		if err := h.ingestor.Ingest(r.Context()); err != nil {
			writeError(w, err)
			return
		}
	}
	res, err := h.splunk.Query(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type savedQueryJSON struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	Query     string `json:"query"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func mapSavedQuery(q sqlite.SavedQuery) savedQueryJSON {
	out := savedQueryJSON{
		ID:     q.ID,
		Name:   q.Name,
		Source: q.Source,
		Query:  q.Query,
	}
	if !q.CreatedAt.IsZero() {
		out.CreatedAt = q.CreatedAt.UTC().Format(time.RFC3339)
	}
	if !q.UpdatedAt.IsZero() {
		out.UpdatedAt = q.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// ListSavedQueries handles GET /query/saved.
func (h *ObservabilityHandler) ListSavedQueries(w http.ResponseWriter, r *http.Request) {
	queries, err := h.splunk.ListSavedQueries(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if queries == nil {
		queries = []sqlite.SavedQuery{}
	}
	out := make([]savedQueryJSON, 0, len(queries))
	for _, q := range queries {
		out = append(out, mapSavedQuery(q))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"queries": out})
}

// SaveQuery handles POST /query/saved.
func (h *ObservabilityHandler) SaveQuery(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Source string `json:"source"`
		Query  string `json:"q"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	q, err := h.splunk.SaveQuery(r.Context(), body.Name, body.Source, body.Query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapSavedQuery(q))
}

// UpdateSavedQuery handles PATCH /query/saved/{id}.
func (h *ObservabilityHandler) UpdateSavedQuery(w http.ResponseWriter, r *http.Request) {
	id, err := resourceID(r, "saved")
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Name   string `json:"name"`
		Source string `json:"source"`
		Q      string `json:"q"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	q, err := h.splunk.UpdateSavedQuery(r.Context(), id, body.Name, body.Source, body.Q)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapSavedQuery(q))
}

// DeleteSavedQuery handles DELETE /query/saved/{id}.
func (h *ObservabilityHandler) DeleteSavedQuery(w http.ResponseWriter, r *http.Request) {
	id, err := resourceID(r, "saved")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.splunk.DeleteSavedQuery(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

// Tail handles GET /query/tail as a Server-Sent Events stream of QueryEvent.
func (h *ObservabilityHandler) Tail(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	site := r.URL.Query().Get("site")

	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if flusher != nil {
		flusher.Flush()
	}

	ctx, cancel := contextWithClient(r)
	defer cancel()

	if h.ingestor != nil {
		if err := h.ingestor.Ingest(ctx); err != nil {
			emitSSEError(w, flusher, err)
			return
		}
	}

	ch := make(chan splunklite.QueryEvent, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- h.splunk.Tail(ctx, source, site, ch)
	}()

	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		case ev, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		case err := <-errCh:
			if err != nil {
				emitSSEError(w, flusher, err)
			}
			return
		}
	}
}

func emitSSEError(w http.ResponseWriter, flusher http.Flusher, err error) {
	appErr := apperror.From(err)
	payload, _ := json.Marshal(appErr.Body())
	_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", payload)
	if flusher != nil {
		flusher.Flush()
	}
}

// TrafficSeries handles GET /metrics/traffic/series.
func (h *ObservabilityHandler) TrafficSeries(w http.ResponseWriter, r *http.Request) {
	rangeSpec := r.URL.Query().Get("range")
	site := r.URL.Query().Get("site")
	res, err := h.grafana.TrafficSeries(r.Context(), rangeSpec, site)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// TrafficTopSites handles GET /metrics/traffic/top-sites.
func (h *ObservabilityHandler) TrafficTopSites(w http.ResponseWriter, r *http.Request) {
	rangeSpec := r.URL.Query().Get("range")
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.grafana.TopSites(r.Context(), rangeSpec, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sites": rows})
}

// TrafficStatusCodes handles GET /metrics/traffic/status-codes.
func (h *ObservabilityHandler) TrafficStatusCodes(w http.ResponseWriter, r *http.Request) {
	rangeSpec := r.URL.Query().Get("range")
	site := r.URL.Query().Get("site")
	res, err := h.grafana.StatusCodes(r.Context(), rangeSpec, site)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// TrafficSummary handles GET /metrics/traffic/summary.
func (h *ObservabilityHandler) TrafficSummary(w http.ResponseWriter, r *http.Request) {
	rangeSpec := r.URL.Query().Get("range")
	res, err := h.grafana.Summary(r.Context(), rangeSpec)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// contextWithClient returns a child context that is cancelled both when the
// request context is done AND when the client disconnects.
func contextWithClient(r *http.Request) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(r.Context())
	if rc, ok := r.Body.(interface{ CloseNotify() <-chan bool }); ok {
		ch := rc.CloseNotify()
		go func() {
			select {
			case <-ch:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	return ctx, cancel
}
