package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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
	h.query(w, r, req)
}

// QueryGet handles GET /query. It supports batch JSON by default and one-shot
// streams when the client requests text/event-stream, application/x-ndjson, or
// passes stream=sse|ndjson.
func (h *ObservabilityHandler) QueryGet(w http.ResponseWriter, r *http.Request) {
	req, err := queryRequestFromURL(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if mode := queryStreamMode(r); mode != "" {
		h.queryStream(w, r, req, mode)
		return
	}
	h.query(w, r, req)
}

func (h *ObservabilityHandler) query(w http.ResponseWriter, r *http.Request, req splunklite.QueryRequest) {
	started := time.Now()
	ingestMs := int64(0)
	queryMs := int64(0)
	status := "ok"
	defer func() {
		slog.Info("observability query",
			"status", status,
			"source", req.Source,
			"q", req.Query,
			"site", req.Site,
			"limit", req.Limit,
			"offset", req.Offset,
			"ingest_ms", ingestMs,
			"query_ms", queryMs,
			"total_ms", time.Since(started).Milliseconds(),
		)
	}()

	if h.ingestor != nil && shouldIngestLogs(req.Source) {
		phase := time.Now()
		if err := h.ingestor.Ingest(r.Context()); err != nil {
			ingestMs = time.Since(phase).Milliseconds()
			status = "ingest_error"
			writeError(w, err)
			return
		}
		ingestMs = time.Since(phase).Milliseconds()
	}
	phase := time.Now()
	res, err := h.splunk.Query(r.Context(), req)
	queryMs = time.Since(phase).Milliseconds()
	if err != nil {
		status = "query_error"
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func shouldIngestLogs(source string) bool {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", splunklite.SourceAll, splunklite.SourceAccess, splunklite.SourceError:
		return true
	default:
		return false
	}
}

type queryStreamFrame struct {
	Type  string                 `json:"type"`
	Hits  int                    `json:"hits,omitempty"`
	Event *splunklite.QueryEvent `json:"event,omitempty"`
	Error interface{}            `json:"error,omitempty"`
}

func (h *ObservabilityHandler) queryStream(w http.ResponseWriter, r *http.Request, req splunklite.QueryRequest, mode string) {
	flusher, _ := w.(http.Flusher)
	if mode == "ndjson" {
		w.Header().Set("Content-Type", "application/x-ndjson")
	} else {
		w.Header().Set("Content-Type", "text/event-stream")
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	emit := func(frame queryStreamFrame) bool {
		data, err := json.Marshal(frame)
		if err != nil {
			return true
		}
		if mode == "ndjson" {
			_, err = fmt.Fprintf(w, "%s\n", data)
		} else if frame.Type == "error" {
			_, err = fmt.Fprintf(w, "event: error\ndata: %s\n\n", data)
		} else {
			_, err = fmt.Fprintf(w, "data: %s\n\n", data)
		}
		if err != nil {
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return true
	}

	if h.ingestor != nil && shouldIngestLogs(req.Source) {
		if !emit(queryStreamFrame{Type: "ingesting"}) {
			return
		}
		if err := h.ingestor.Ingest(r.Context()); err != nil {
			emit(queryStreamFrame{Type: "error", Error: apperror.From(err).Body().Error})
			return
		}
	}
	res, err := h.splunk.Query(r.Context(), req)
	if err != nil {
		emit(queryStreamFrame{Type: "error", Error: apperror.From(err).Body().Error})
		return
	}
	if !emit(queryStreamFrame{Type: "meta", Hits: res.Hits}) {
		return
	}
	for _, ev := range res.Events {
		event := ev
		if !emit(queryStreamFrame{Type: "event", Event: &event}) {
			return
		}
	}
	emit(queryStreamFrame{Type: "done"})
}

func queryRequestFromURL(r *http.Request) (splunklite.QueryRequest, error) {
	q := r.URL.Query()
	req := splunklite.QueryRequest{
		Source: q.Get("source"),
		Site:   q.Get("site"),
		Query:  q.Get("q"),
		Limit:  queryInt(r, "limit", 100),
		Offset: queryInt(r, "offset", 0),
	}
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		from, err := parseQueryTime(raw)
		if err != nil {
			return req, apperror.Wrap(apperror.CodeInvalidInput, "invalid from", err)
		}
		req.From = &from
	}
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		to, err := parseQueryTime(raw)
		if err != nil {
			return req, apperror.Wrap(apperror.CodeInvalidInput, "invalid to", err)
		}
		req.To = &to
	}
	return req, nil
}

func parseQueryTime(raw string) (time.Time, error) {
	if strings.EqualFold(raw, "now") {
		return time.Now().UTC(), nil
	}
	return time.Parse(time.RFC3339, raw)
}

func queryStreamMode(r *http.Request) string {
	s := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("stream")))
	if s == "sse" || s == "1" || s == "true" {
		return "sse"
	}
	if s == "ndjson" || s == "jsonl" {
		return "ndjson"
	}
	accept := strings.ToLower(r.Header.Get("Accept"))
	switch {
	case strings.Contains(accept, "application/x-ndjson") || strings.Contains(accept, "application/jsonl"):
		return "ndjson"
	case strings.Contains(accept, "text/event-stream"):
		return "sse"
	default:
		return ""
	}
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

	// Long-lived SSE: the server's WriteTimeout would otherwise close the
	// connection after the global default once the stream goes idle. Clear
	// the write deadline so Tail can stay open for the entire browser
	// session. ReadTimeout is irrelevant for SSE since we never read the
	// body after the request is dispatched.
	if rc, ok := w.(interface {
		SetWriteDeadline(time.Time) error
	}); ok {
		_ = rc.SetWriteDeadline(time.Time{})
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
