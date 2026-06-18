package splunklite

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

const (
	SourceAudit  = "audit"
	SourceJob    = "job"
	SourceAccess = "access"
	SourceError  = "error"
	SourceAll    = "all"
)

// QueryRequest is the POST /query body.
type QueryRequest struct {
	Source string     `json:"source"`
	Site   string     `json:"site,omitempty"`
	Query  string     `json:"q"`
	From   *time.Time `json:"from"`
	To     *time.Time `json:"to"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// QueryEvent is a normalized hit returned to clients.
type QueryEvent struct {
	// ID is a stable composite key (source|ts|message-hash) used by the
	// frontend to dedup rows when the SSE stream replays its backlog after
	// a reconnect.
	ID      string                 `json:"id"`
	TS      time.Time              `json:"ts"`
	Source  string                 `json:"source"`
	Action  string                 `json:"action"`
	User    string                 `json:"user"`
	Message string                 `json:"message"`
	Meta    map[string]interface{} `json:"meta"`
}

// QueryResult is the POST /query response.
type QueryResult struct {
	Hits   int          `json:"hits"`
	Events []QueryEvent `json:"events"`
}

// Service executes Splunk Lite queries across SQLite sources.
type Service struct {
	audit    *sqlite.AuditRepository
	jobs     *sqlite.JobRepository
	logs     *sqlite.LogEventRepository
	saved    *sqlite.SavedQueryRepository
	ingestor *LogIngestor
	auditTTL int
	logTTL   int

	// Per-source watermark of the last event TS pushed to a /query/tail
	// subscriber. Persisted across SSE reconnects so a brief network blip
	// does NOT cause the tail to replay every event since the dawn of time.
	tailMu     sync.Mutex
	tailCursor map[string]time.Time
}

// NewService returns a Splunk Lite query service.
func NewService(
	audit *sqlite.AuditRepository,
	jobs *sqlite.JobRepository,
	logs *sqlite.LogEventRepository,
	saved *sqlite.SavedQueryRepository,
	auditRetentionDays, logRetentionDays int,
) *Service {
	if auditRetentionDays <= 0 {
		auditRetentionDays = 90
	}
	if logRetentionDays <= 0 {
		logRetentionDays = 14
	}
	return &Service{
		audit:      audit,
		jobs:       jobs,
		logs:       logs,
		saved:      saved,
		auditTTL:   auditRetentionDays,
		logTTL:     logRetentionDays,
		tailCursor: make(map[string]time.Time),
	}
}

// SetIngestor attaches a log ingestor used by Tail/Query to backfill.
func (s *Service) SetIngestor(i *LogIngestor) {
	s.ingestor = i
}

// AuditWriter implements contracts.AuditWriter using the audit repository.
type AuditWriter struct {
	repo *sqlite.AuditRepository
}

// NewAuditWriter returns an audit writer backed by repo.
func NewAuditWriter(repo *sqlite.AuditRepository) *AuditWriter {
	return &AuditWriter{repo: repo}
}

// Write persists a single audit entry.
func (w *AuditWriter) Write(ctx context.Context, entry contracts.AuditEntry) error {
	if w == nil || w.repo == nil {
		return fmt.Errorf("audit writer not configured")
	}
	return w.repo.Write(ctx, sqlite.AuditLog{
		Timestamp:    entry.Timestamp,
		UserEmail:    entry.UserEmail,
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		Domain:       entry.Domain,
		Status:       entry.Status,
		Message:      entry.Message,
		MetaJSON:     entry.MetaJSON,
	})
}

// Query executes a structured search request.
func (s *Service) Query(ctx context.Context, req QueryRequest) (QueryResult, error) {
	if err := validateTimeRange(req.From, req.To); err != nil {
		return QueryResult{}, err
	}
	filter, pipes, err := ParseSearch(req.Query)
	if err != nil {
		return QueryResult{}, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "" {
		source = SourceAll
	}

	var events []QueryEvent
	var totalHits int

	switch source {
	case SourceAudit:
		events, totalHits, err = s.queryAudit(ctx, req, filter, limit)
	case SourceJob:
		events, totalHits, err = s.queryJobs(ctx, req, filter, limit)
	case SourceAccess:
		events, totalHits, err = s.queryLogs(ctx, req, sqlite.LogSourceAccess, filter, limit)
	case SourceError:
		events, totalHits, err = s.queryLogs(ctx, req, sqlite.LogSourceError, filter, limit)
	case SourceAll:
		events, totalHits, err = s.queryAll(ctx, req, filter, pipes, limit)
	default:
		return QueryResult{}, apperror.New(apperror.CodeQueryInvalid, "unknown source")
	}
	if err != nil {
		return QueryResult{}, err
	}

	if source != SourceAll {
		events = ApplyPipes(events, pipes)
		for _, pipe := range pipes {
			if pipe.Name == "head" {
				totalHits = len(events)
				break
			}
		}
	}

	return QueryResult{Hits: totalHits, Events: events}, nil
}

func validateTimeRange(from, to *time.Time) error {
	if from != nil && to != nil && from.After(*to) {
		return apperror.New(apperror.CodeTimeRangeInvalid, "from must be before to")
	}
	return nil
}

func (s *Service) queryAudit(ctx context.Context, req QueryRequest, filter FilterExpr, limit int) ([]QueryEvent, int, error) {
	sqlFilter, err := CompileFilter(filter, SourceKindAudit)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}
	if !sqlFilter.Applicable {
		return nil, 0, nil
	}
	auditFilter := sqlite.AuditFilter{
		From: req.From, To: req.To,
		Wheres: sqlFilter.Wheres, Args: sqlFilter.Args,
		Limit: limit, Offset: req.Offset,
	}
	count, err := s.audit.Count(ctx, auditFilter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "count audit logs", err)
	}
	rows, err := s.audit.List(ctx, auditFilter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "list audit logs", err)
	}
	return mapAuditEvents(rows), count, nil
}

func (s *Service) queryJobs(ctx context.Context, req QueryRequest, filter FilterExpr, limit int) ([]QueryEvent, int, error) {
	sqlFilter, err := CompileFilter(filter, SourceKindJob)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}
	if !sqlFilter.Applicable {
		return nil, 0, nil
	}
	jobFilter := sqlite.JobFilter{
		From: req.From, To: req.To,
		Wheres: sqlFilter.Wheres, Args: sqlFilter.Args,
		Limit: limit, Offset: req.Offset,
	}
	count, err := s.jobs.Count(ctx, jobFilter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "count job runs", err)
	}
	rows, err := s.jobs.List(ctx, jobFilter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "list job runs", err)
	}
	return mapJobEvents(rows), count, nil
}

func (s *Service) queryLogs(ctx context.Context, req QueryRequest, logSource string, filter FilterExpr, limit int) ([]QueryEvent, int, error) {
	kind := sourceKindForLog(logSource)
	filter = WithSiteScope(filter, req.Site)
	sqlFilter, err := CompileFilter(filter, kind)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}
	if !sqlFilter.Applicable {
		return nil, 0, nil
	}
	logFilter := sqlite.LogEventFilter{
		Source: logSource,
		From:   req.From, To: req.To,
		Wheres: sqlFilter.Wheres, Args: sqlFilter.Args,
		Limit: limit, Offset: req.Offset,
	}
	count, err := s.logs.Count(ctx, logFilter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "count log events", err)
	}
	rows, err := s.logs.List(ctx, logFilter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "list log events", err)
	}
	return mapLogEvents(rows, logSource), count, nil
}

func (s *Service) queryAll(ctx context.Context, req QueryRequest, filter FilterExpr, pipes []PipeCmd, limit int) ([]QueryEvent, int, error) {
	fetch := limit + req.Offset
	if fetch <= 0 {
		fetch = 100
	}
	if fetch > 2000 {
		fetch = 2000
	}

	type result struct {
		events []QueryEvent
		hits   int
		err    error
	}
	sources := []string{SourceAudit, SourceJob, SourceAccess, SourceError}
	ch := make(chan result, len(sources))

	for _, src := range sources {
		src := src
		go func() {
			sub := req
			sub.Offset = 0
			var (
				events []QueryEvent
				hits   int
				err    error
			)
			switch src {
			case SourceAudit:
				events, hits, err = s.queryAudit(ctx, sub, filter, fetch)
			case SourceJob:
				events, hits, err = s.queryJobs(ctx, sub, filter, fetch)
			case SourceAccess:
				events, hits, err = s.queryLogs(ctx, sub, sqlite.LogSourceAccess, filter, fetch)
			case SourceError:
				events, hits, err = s.queryLogs(ctx, sub, sqlite.LogSourceError, filter, fetch)
			}
			ch <- result{events: events, hits: hits, err: err}
		}()
	}

	merged := make([]QueryEvent, 0, fetch)
	for range sources {
		res := <-ch
		if res.err != nil {
			return nil, 0, res.err
		}
		merged = append(merged, res.events...)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].TS.After(merged[j].TS)
	})
	start := req.Offset
	if start > len(merged) {
		start = len(merged)
	}
	end := start + limit
	if end > len(merged) {
		end = len(merged)
	}
	sliced := merged[start:end]
	sliced = ApplyPipes(sliced, pipes)
	return sliced, len(sliced), nil
}

func mapAuditEvents(rows []sqlite.AuditLog) []QueryEvent {
	out := make([]QueryEvent, 0, len(rows))
	for _, row := range rows {
		meta := map[string]interface{}{}
		if row.MetaJSON != "" && row.MetaJSON != "{}" {
			_ = json.Unmarshal([]byte(row.MetaJSON), &meta)
		}
		meta["resource_type"] = row.ResourceType
		meta["resource_id"] = row.ResourceID
		meta["domain"] = row.Domain
		meta["status"] = row.Status
		out = append(out, QueryEvent{
			ID:      eventID(SourceAudit, row.Timestamp, row.Message),
			TS:      row.Timestamp,
			Source:  SourceAudit,
			Action:  row.Action,
			User:    row.UserEmail,
			Message: row.Message,
			Meta:    meta,
		})
	}
	return out
}

func mapJobEvents(rows []sqlite.JobRun) []QueryEvent {
	out := make([]QueryEvent, 0, len(rows))
	for _, row := range rows {
		msg := row.Name
		if row.Error != "" {
			msg = row.Error
		} else if row.Output != "" {
			msg = row.Output
		}
		out = append(out, QueryEvent{
			ID:      eventID(SourceJob, row.CreatedAt, msg),
			TS:      row.CreatedAt,
			Source:  SourceJob,
			Action:  row.JobType,
			User:    "",
			Message: msg,
			Meta: map[string]interface{}{
				"name":   row.Name,
				"status": row.Status,
			},
		})
	}
	return out
}

func mapLogEvents(rows []sqlite.LogEvent, source string) []QueryEvent {
	out := make([]QueryEvent, 0, len(rows))
	for _, row := range rows {
		meta := map[string]interface{}{
			"site": row.Site,
		}
		if row.StatusCode != nil {
			meta["status_code"] = *row.StatusCode
		}
		if row.Bytes != nil {
			meta["bytes"] = *row.Bytes
		}
		out = append(out, QueryEvent{
			ID:      eventID(source, row.Timestamp, row.RawPreview),
			TS:      row.Timestamp,
			Source:  source,
			Action:  "log.line",
			User:    "",
			Message: row.RawPreview,
			Meta:    meta,
		})
	}
	return out
}

// eventID derives a stable, compact id for an event so the frontend can
// dedup across reconnects. The timestamp is rendered in nanos so two
// events one nanosecond apart never collide, and the first 64 chars of
// the message disambiguate bursts with the same ts.
func eventID(source string, ts time.Time, message string) string {
	msg := message
	if len(msg) > 64 {
		msg = msg[:64]
	}
	return source + "|" + ts.UTC().Format(time.RFC3339Nano) + "|" + msg
}

// ListSavedQueries returns stored queries.
func (s *Service) ListSavedQueries(ctx context.Context) ([]sqlite.SavedQuery, error) {
	if s.saved == nil {
		return nil, nil
	}
	return s.saved.List(ctx)
}

// SaveQuery stores a named query.
func (s *Service) SaveQuery(ctx context.Context, name, source, query string) (sqlite.SavedQuery, error) {
	if strings.TrimSpace(name) == "" {
		return sqlite.SavedQuery{}, apperror.New(apperror.CodeInvalidInput, "name required")
	}
	if _, _, err := ParseSearch(query); err != nil && strings.TrimSpace(query) != "" {
		return sqlite.SavedQuery{}, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}
	return s.saved.Create(ctx, name, source, query)
}

// RecentAudit returns latest audit events for dashboard use.
func (s *Service) RecentAudit(ctx context.Context, limit int) ([]QueryEvent, error) {
	rows, err := s.audit.Recent(ctx, limit)
	if err != nil {
		return nil, err
	}
	return mapAuditEvents(rows), nil
}

// PurgeRetention removes expired audit and log event rows.
func (s *Service) PurgeRetention(ctx context.Context, now time.Time) error {
	if s.audit != nil && s.auditTTL > 0 {
		cutoff := now.Add(-time.Duration(s.auditTTL) * 24 * time.Hour)
		if _, err := s.audit.PurgeOlderThan(ctx, cutoff); err != nil {
			return err
		}
	}
	if s.logs != nil && s.logTTL > 0 {
		cutoff := now.Add(-time.Duration(s.logTTL) * 24 * time.Hour)
		if _, err := s.logs.PurgeOlderThan(ctx, cutoff); err != nil {
			return err
		}
	}
	return nil
}

// UpdateSavedQuery partially updates a saved query by id. Empty fields are
// ignored. The new query, if provided, is validated via ParseSearch.
func (s *Service) UpdateSavedQuery(ctx context.Context, id int64, name, source, query string) (sqlite.SavedQuery, error) {
	if s.saved == nil {
		return sqlite.SavedQuery{}, apperror.New(apperror.CodeInternal, "saved query repository not configured")
	}
	if query != "" {
		if _, _, err := ParseSearch(query); err != nil {
			return sqlite.SavedQuery{}, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
		}
	}
	updated, err := s.saved.Update(ctx, id, name, source, query)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") || strings.Contains(err.Error(), "not found") {
			return sqlite.SavedQuery{}, apperror.New(apperror.CodeNotFound, "saved query not found")
		}
		return sqlite.SavedQuery{}, apperror.Wrap(apperror.CodeDatabase, "update saved query", err)
	}
	return updated, nil
}

// DeleteSavedQuery removes a saved query by id. Returns CodeNotFound if the
// row doesn't exist.
func (s *Service) DeleteSavedQuery(ctx context.Context, id int64) error {
	if s.saved == nil {
		return apperror.New(apperror.CodeInternal, "saved query repository not configured")
	}
	if err := s.saved.Delete(ctx, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apperror.New(apperror.CodeNotFound, "saved query not found")
		}
		return apperror.Wrap(apperror.CodeDatabase, "delete saved query", err)
	}
	return nil
}

// Tail polls the underlying repositories for new rows and pushes them onto ch.
// It calls Ingestor.Ingest once at the start to backfill missing log lines.
// source may be one of: audit, job, access, error, all (default).
// site is optional and only applies to access/error sources.
//
// The high-water mark per (source, site) is persisted in the Service struct
// across SSE reconnects. To stay true to "tail -f" semantics the cursor is
// reset to the current wall clock whenever the gap between the last cursor
// and the current time exceeds `tailReconnectGap`, so a long disconnect does
// not flood the client with old events on reconnect.
//
// If the browser reconnects with a Last-Event-ID (resumeFrom), the cursor is
// pinned to the timestamp embedded in that id and any backlog since then is
// re-emitted. The id format is "source|ts|message"; only the timestamp is
// used for resumption.
func (s *Service) Tail(ctx context.Context, source, site string, ch chan<- QueryEvent, resumeFrom string) error {
	return s.TailQuery(ctx, source, site, "", ch, resumeFrom)
}

// TailQuery is Tail with the same mini-query filter used by /query.
func (s *Service) TailQuery(ctx context.Context, source, site, query string, ch chan<- QueryEvent, resumeFrom string) error {
	if s.ingestor != nil {
		if err := s.ingestor.Ingest(ctx); err != nil {
			return err
		}
	}
	filter, _, err := ParseSearch(query)
	if err != nil {
		return apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = SourceAll
	}

	sources := []string{source}
	if source == SourceAll {
		sources = []string{SourceAudit, SourceJob, SourceAccess, SourceError}
	}

	// Parse the resume token (a Last-Event-ID from the browser) once and
	// use its timestamp as the cursor for every relevant source key.
	var resumeTS time.Time
	if resumeFrom != "" {
		if ts, ok := parseTailIDTime(resumeFrom); ok {
			resumeTS = ts
		}
	}

	// Seed / re-seed per-key watermarks. First connect for a key: pin to
	// time.Now() so we only stream events that arrive from now on.
	// Reconnect after a long gap: also reset to time.Now() to skip the
	// backlog (live-tail semantics, not history-replay).
	// Reconnect with Last-Event-ID: resume from the embedded timestamp
	// (with a tiny epsilon to avoid re-emitting the boundary event).
	now := time.Now().UTC()
	s.tailMu.Lock()
	for _, src := range sources {
		key := tailKey(src, site)
		switch {
		case !resumeTS.IsZero():
			s.tailCursor[key] = resumeTS.Add(time.Nanosecond)
		default:
			prev, ok := s.tailCursor[key]
			if !ok || now.Sub(prev) > tailReconnectGap {
				s.tailCursor[key] = now
			}
		}
	}
	s.tailMu.Unlock()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, src := range sources {
				s.tailMu.Lock()
				since := s.tailCursor[tailKey(src, site)]
				s.tailMu.Unlock()
				maxTS, err := s.tailOne(ctx, src, site, since, filter, ch)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
				if !maxTS.IsZero() && maxTS.After(since) {
					s.tailMu.Lock()
					s.tailCursor[tailKey(src, site)] = maxTS
					s.tailMu.Unlock()
				}
			}
		}
	}
}

// parseTailIDTime extracts the timestamp portion of an event id of the form
// "source|RFC3339Nano|message". Returns false if the id does not match.
func parseTailIDTime(id string) (time.Time, bool) {
	first := strings.IndexByte(id, '|')
	if first < 0 {
		return time.Time{}, false
	}
	rest := id[first+1:]
	second := strings.IndexByte(rest, '|')
	if second < 0 {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, rest[:second])
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func tailKey(source, site string) string {
	return source + "\x00" + site
}

// tailReconnectGap caps how much historical gap a live tail is willing to
// replay after a brief network blip. Beyond this threshold, the tail
// resumes from time.Now() and skips the backlog (mirrors `tail -f`).
const tailReconnectGap = 30 * time.Second

// tailOne queries the named source for rows newer than `since`, maps them to
// QueryEvents, sends them on ch, and returns the max ts observed.
func (s *Service) tailOne(ctx context.Context, source, site string, since time.Time, filter FilterExpr, ch chan<- QueryEvent) (time.Time, error) {
	var maxTS time.Time
	kind := sourceKindForName(source)
	if source == SourceAccess || source == SourceError {
		kind = sourceKindForLog(source)
		filter = WithSiteScope(filter, site)
	}
	record := func(ev QueryEvent) bool {
		if ev.TS.After(maxTS) {
			maxTS = ev.TS
		}
		if !EvalFilter(filter, ev, kind) {
			return true
		}
		return sendCtx(ctx, ch, ev)
	}
	switch source {
	case SourceAudit:
		if s.audit == nil {
			return maxTS, nil
		}
		rows, err := s.audit.ListSince(ctx, since)
		if err != nil {
			return maxTS, err
		}
		for _, ev := range mapAuditEvents(rows) {
			if !record(ev) {
				return maxTS, nil
			}
		}
	case SourceJob:
		if s.jobs == nil {
			return maxTS, nil
		}
		rows, err := s.jobs.ListSince(ctx, since)
		if err != nil {
			return maxTS, err
		}
		for _, ev := range mapJobEvents(rows) {
			if !record(ev) {
				return maxTS, nil
			}
		}
	case SourceAccess, SourceError:
		if s.logs == nil {
			return maxTS, nil
		}
		rows, err := s.logs.ListSince(ctx, source, site, since)
		if err != nil {
			return maxTS, err
		}
		for _, ev := range mapLogEvents(rows, source) {
			if !record(ev) {
				return maxTS, nil
			}
		}
	}
	return maxTS, nil
}

// sendCtx pushes ev to ch or returns false if the context was cancelled.
func sendCtx(ctx context.Context, ch chan<- QueryEvent, ev QueryEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- ev:
		return true
	}
}
