package splunklite

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
		audit:    audit,
		jobs:     jobs,
		logs:     logs,
		saved:    saved,
		auditTTL: auditRetentionDays,
		logTTL:   logRetentionDays,
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
	clauses, err := ParseQuery(req.Query)
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
		events, totalHits, err = s.queryAudit(ctx, req, clauses, limit)
	case SourceJob:
		events, totalHits, err = s.queryJobs(ctx, req, clauses, limit)
	case SourceAccess:
		events, totalHits, err = s.queryLogs(ctx, req, sqlite.LogSourceAccess, clauses, limit)
	case SourceError:
		events, totalHits, err = s.queryLogs(ctx, req, sqlite.LogSourceError, clauses, limit)
	case SourceAll:
		events, totalHits, err = s.queryAll(ctx, req, clauses, limit)
	default:
		return QueryResult{}, apperror.New(apperror.CodeQueryInvalid, "unknown source")
	}
	if err != nil {
		return QueryResult{}, err
	}

	return QueryResult{Hits: totalHits, Events: events}, nil
}

func validateTimeRange(from, to *time.Time) error {
	if from != nil && to != nil && from.After(*to) {
		return apperror.New(apperror.CodeTimeRangeInvalid, "from must be before to")
	}
	return nil
}

func (s *Service) queryAudit(ctx context.Context, req QueryRequest, clauses []sqlite.FieldClause, limit int) ([]QueryEvent, int, error) {
	wheres, args, err := sqlite.BuildAuditWhere(clauses)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}
	filter := sqlite.AuditFilter{
		From: req.From, To: req.To,
		Wheres: wheres, Args: args,
		Limit: limit, Offset: req.Offset,
	}
	count, err := s.audit.Count(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "count audit logs", err)
	}
	rows, err := s.audit.List(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "list audit logs", err)
	}
	return mapAuditEvents(rows), count, nil
}

func (s *Service) queryJobs(ctx context.Context, req QueryRequest, clauses []sqlite.FieldClause, limit int) ([]QueryEvent, int, error) {
	wheres, args, err := buildJobWhere(clauses)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}
	filter := sqlite.JobFilter{
		From: req.From, To: req.To,
		Wheres: wheres, Args: args,
		Limit: limit, Offset: req.Offset,
	}
	count, err := s.jobs.Count(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "count job runs", err)
	}
	rows, err := s.jobs.List(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "list job runs", err)
	}
	return mapJobEvents(rows), count, nil
}

func (s *Service) queryLogs(ctx context.Context, req QueryRequest, logSource string, clauses []sqlite.FieldClause, limit int) ([]QueryEvent, int, error) {
	clauses = applySiteScope(clauses, req.Site)
	wheres, args, err := sqlite.BuildLogEventWhere(clauses)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeQueryInvalid, err.Error(), err)
	}
	filter := sqlite.LogEventFilter{
		Source: logSource,
		From:   req.From, To: req.To,
		Wheres: wheres, Args: args,
		Limit: limit, Offset: req.Offset,
	}
	count, err := s.logs.Count(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "count log events", err)
	}
	rows, err := s.logs.List(ctx, filter)
	if err != nil {
		return nil, 0, apperror.Wrap(apperror.CodeDatabase, "list log events", err)
	}
	return mapLogEvents(rows, logSource), count, nil
}

func applySiteScope(clauses []sqlite.FieldClause, site string) []sqlite.FieldClause {
	site = strings.TrimSpace(site)
	if site == "" {
		return clauses
	}
	out := make([]sqlite.FieldClause, 0, len(clauses)+1)
	out = append(out, clauses...)
	out = append(out, sqlite.LikeClause("site", site))
	return out
}

func (s *Service) queryAll(ctx context.Context, req QueryRequest, clauses []sqlite.FieldClause, limit int) ([]QueryEvent, int, error) {
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
			sub.Source = src
			sub.Limit = fetch
			sub.Offset = 0
			res, err := s.Query(ctx, sub)
			ch <- result{events: res.Events, hits: res.Hits, err: err}
		}()
	}

	merged := make([]QueryEvent, 0, fetch)
	total := 0
	for range sources {
		res := <-ch
		if res.err != nil {
			return nil, 0, res.err
		}
		total += res.hits
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
	return merged[start:end], total, nil
}
func buildJobWhere(clauses []sqlite.FieldClause) (wheres []string, args []interface{}, err error) {
	columns := map[string]string{
		"job_type": "job_type",
		"type":     "job_type",
		"name":     "name",
		"status":   "status",
		"output":   "output",
		"error":    "error",
		"message":  "name",
	}
	for _, c := range clauses {
		if c.Field == "_text" {
			if c.Kind == sqlite.RegexpClauseKind {
				w, a := sqlite.BuildFreeTextRegexpWhere([]string{"name", "output", "error", "job_type"}, c.Value)
				wheres = append(wheres, w)
				args = append(args, a...)
				continue
			}
			w, a := sqlite.BuildFreeTextWhere([]string{"name", "output", "error", "job_type"}, c.Value)
			wheres = append(wheres, w)
			args = append(args, a...)
			continue
		}
		col, ok := columns[c.Field]
		if !ok {
			return nil, nil, fmt.Errorf("unknown job field %q", c.Field)
		}
		if c.Kind == sqlite.RegexpClauseKind {
			wheres = append(wheres, col+` REGEXP ?`)
			args = append(args, c.Value)
			continue
		}
		w, a := sqliteLike(col, c.Value)
		wheres = append(wheres, w)
		args = append(args, a)
	}
	return wheres, args, nil
}

func sqliteLike(column, pattern string) (string, interface{}) {
	if strings.Contains(pattern, "*") {
		escaped := strings.ReplaceAll(pattern, `%`, `\%`)
		escaped = strings.ReplaceAll(escaped, `_`, `\_`)
		escaped = strings.ReplaceAll(escaped, `*`, `%`)
		return column + ` LIKE ? ESCAPE '\'`, escaped
	}
	return column + ` = ?`, pattern
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
	if _, err := ParseQuery(query); err != nil && strings.TrimSpace(query) != "" {
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
// ignored. The new query, if provided, is validated via ParseQuery.
func (s *Service) UpdateSavedQuery(ctx context.Context, id int64, name, source, query string) (sqlite.SavedQuery, error) {
	if s.saved == nil {
		return sqlite.SavedQuery{}, apperror.New(apperror.CodeInternal, "saved query repository not configured")
	}
	if query != "" {
		if _, err := ParseQuery(query); err != nil {
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
func (s *Service) Tail(ctx context.Context, source, site string, ch chan<- QueryEvent) error {
	if s.ingestor != nil {
		if err := s.ingestor.Ingest(ctx); err != nil {
			return err
		}
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		source = SourceAll
	}

	sources := []string{source}
	if source == SourceAll {
		sources = []string{SourceAudit, SourceJob, SourceAccess, SourceError}
	}

	// Track the most recent ts seen per source so we only emit new rows.
	last := map[string]time.Time{}
	for _, src := range sources {
		last[src] = time.Time{}
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, src := range sources {
				maxTS, err := s.tailOne(ctx, src, site, last[src], ch)
				if err != nil {
					// ignore context cancellation as success
					if ctx.Err() != nil {
						return nil
					}
					return err
				}
				if !maxTS.IsZero() && maxTS.After(last[src]) {
					last[src] = maxTS
				}
			}
		}
	}
}

// tailOne queries the named source for rows newer than `since`, maps them to
// QueryEvents, sends them on ch, and returns the max ts observed.
func (s *Service) tailOne(ctx context.Context, source, site string, since time.Time, ch chan<- QueryEvent) (time.Time, error) {
	var maxTS time.Time
	record := func(ev QueryEvent) bool {
		if ev.TS.After(maxTS) {
			maxTS = ev.TS
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
