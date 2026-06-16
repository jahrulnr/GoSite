package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	LogSourceAccess = "access"
	LogSourceError  = "error"
)

// LogEvent is an ingested nginx log line summary.
type LogEvent struct {
	ID         int64
	Timestamp  time.Time
	Source     string
	Site       string
	StatusCode *int
	Bytes      *int
	LineHash   string
	RawPreview string
}

// LogEventRepository persists log event rows.
type LogEventRepository struct {
	db *sql.DB
}

// NewLogEventRepository returns a log event repository backed by db.
func NewLogEventRepository(db *sql.DB) *LogEventRepository {
	return &LogEventRepository{db: db}
}

// Insert stores a log event row.
func (r *LogEventRepository) Insert(ctx context.Context, ev LogEvent) (LogEvent, error) {
	ts := ev.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO log_events (ts, source, site, status_code, bytes, line_hash, raw_preview)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, ts, ev.Source, ev.Site, nullInt(ev.StatusCode), nullInt(ev.Bytes), ev.LineHash, ev.RawPreview)
	if err != nil {
		return LogEvent{}, fmt.Errorf("insert log event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return LogEvent{}, fmt.Errorf("last insert id: %w", err)
	}
	ev.ID = id
	ev.Timestamp = ts
	return ev, nil
}

// InsertIgnore stores a log event row and ignores duplicate line hashes.
func (r *LogEventRepository) InsertIgnore(ctx context.Context, ev LogEvent) error {
	ts := ev.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO log_events (ts, source, site, status_code, bytes, line_hash, raw_preview)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, ts, ev.Source, ev.Site, nullInt(ev.StatusCode), nullInt(ev.Bytes), ev.LineHash, ev.RawPreview)
	if err != nil {
		return fmt.Errorf("insert log event: %w", err)
	}
	return nil
}

// InsertIgnoreBatch inserts many log event rows inside a single transaction
// and silently ignores duplicate line hashes. It is significantly faster than
// per-row InsertIgnore when the input is large because it amortizes the
// per-statement commit cost.
func (r *LogEventRepository) InsertIgnoreBatch(ctx context.Context, events []LogEvent) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO log_events (ts, source, site, status_code, bytes, line_hash, raw_preview)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()
	inserted := 0
	for _, ev := range events {
		ts := ev.Timestamp
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		if _, err := stmt.ExecContext(ctx, ts, ev.Source, ev.Site, nullInt(ev.StatusCode), nullInt(ev.Bytes), ev.LineHash, ev.RawPreview); err != nil {
			_ = tx.Rollback()
			return inserted, fmt.Errorf("insert log event: %w", err)
		}
		inserted++
	}
	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("commit: %w", err)
	}
	return inserted, nil
}

// LogEventFilter constrains log event queries.
type LogEventFilter struct {
	Source string
	From   *time.Time
	To     *time.Time
	Wheres []string
	Args   []interface{}
	Limit  int
	Offset int
}

// List returns log events matching filter ordered by ts desc.
func (r *LogEventRepository) List(ctx context.Context, f LogEventFilter) ([]LogEvent, error) {
	query := `SELECT id, ts, source, site, status_code, bytes, line_hash, raw_preview FROM log_events WHERE 1=1`
	args := make([]interface{}, 0, len(f.Args)+4)
	if f.Source != "" {
		query += ` AND source = ?`
		args = append(args, f.Source)
	}
	if f.From != nil {
		query += ` AND ts >= ?`
		args = append(args, *f.From)
	}
	if f.To != nil {
		query += ` AND ts <= ?`
		args = append(args, *f.To)
	}
	for _, w := range f.Wheres {
		query += ` AND ` + w
	}
	args = append(args, f.Args...)
	query += ` ORDER BY ts DESC`
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	if f.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, f.Offset)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list log events: %w", err)
	}
	defer rows.Close()

	var out []LogEvent
	for rows.Next() {
		var ev LogEvent
		var ts sql.NullTime
		var statusCode, bytes sql.NullInt64
		if err := rows.Scan(&ev.ID, &ts, &ev.Source, &ev.Site, &statusCode, &bytes, &ev.LineHash, &ev.RawPreview); err != nil {
			return nil, fmt.Errorf("scan log event: %w", err)
		}
		if ts.Valid {
			ev.Timestamp = ts.Time
		}
		if statusCode.Valid {
			v := int(statusCode.Int64)
			ev.StatusCode = &v
		}
		if bytes.Valid {
			v := int(bytes.Int64)
			ev.Bytes = &v
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// Count returns matching log events.
func (r *LogEventRepository) Count(ctx context.Context, f LogEventFilter) (int, error) {
	query := `SELECT COUNT(1) FROM log_events WHERE 1=1`
	args := make([]interface{}, 0, len(f.Args)+3)
	if f.Source != "" {
		query += ` AND source = ?`
		args = append(args, f.Source)
	}
	if f.From != nil {
		query += ` AND ts >= ?`
		args = append(args, *f.From)
	}
	if f.To != nil {
		query += ` AND ts <= ?`
		args = append(args, *f.To)
	}
	for _, w := range f.Wheres {
		query += ` AND ` + w
	}
	args = append(args, f.Args...)
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count log events: %w", err)
	}
	return count, nil
}

// PurgeOlderThan deletes log events before cutoff.
func (r *LogEventRepository) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM log_events WHERE ts < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge log events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ListSince returns log events with ts > since, optionally filtered by source/site,
// ordered by ts asc.
func (r *LogEventRepository) ListSince(ctx context.Context, source, site string, since time.Time) ([]LogEvent, error) {
	query := `SELECT id, ts, source, site, status_code, bytes, line_hash, raw_preview FROM log_events WHERE ts > ?`
	args := []interface{}{since}
	if source != "" {
		query += ` AND source = ?`
		args = append(args, source)
	}
	if site != "" {
		query += ` AND site = ?`
		args = append(args, site)
	}
	query += ` ORDER BY ts ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list log events since: %w", err)
	}
	defer rows.Close()

	var out []LogEvent
	for rows.Next() {
		var ev LogEvent
		var ts sql.NullTime
		var statusCode, bytes sql.NullInt64
		if err := rows.Scan(&ev.ID, &ts, &ev.Source, &ev.Site, &statusCode, &bytes, &ev.LineHash, &ev.RawPreview); err != nil {
			return nil, fmt.Errorf("scan log event: %w", err)
		}
		if ts.Valid {
			ev.Timestamp = ts.Time
		}
		if statusCode.Valid {
			v := int(statusCode.Int64)
			ev.StatusCode = &v
		}
		if bytes.Valid {
			v := int(bytes.Int64)
			ev.Bytes = &v
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// BuildLogEventWhere converts parsed field clauses into SQL fragments.
// Clauses with `Kind == RegexpClauseKind` emit `column REGEXP ?`; everything
// else is treated as a LIKE/= match.
func BuildLogEventWhere(clauses []FieldClause) (wheres []string, args []interface{}, err error) {
	columns := map[string]string{
		"site":        "site",
		"status":      "status_code",
		"status_code": "status_code",
		"message":     "raw_preview",
		"preview":     "raw_preview",
	}
	for _, c := range clauses {
		if c.Field == "_text" {
			if c.Kind == RegexpClauseKind {
				w, a := BuildFreeTextRegexpWhere([]string{"site", "raw_preview"}, c.Value)
				wheres = append(wheres, w)
				args = append(args, a...)
				continue
			}
			w, a := BuildFreeTextWhere([]string{"site", "raw_preview"}, c.Value)
			wheres = append(wheres, w)
			args = append(args, a...)
			continue
		}
		col, ok := columns[c.Field]
		if !ok {
			return nil, nil, fmt.Errorf("unknown log field %q", c.Field)
		}
		if c.Kind == RegexpClauseKind {
			wheres = append(wheres, col+` REGEXP ?`)
			args = append(args, c.Value)
			continue
		}
		if col == "status_code" && !containsWildcard(c.Value) {
			wheres = append(wheres, col+` = ?`)
			args = append(args, c.Value)
			continue
		}
		w, a := likeClause(col, c.Value)
		wheres = append(wheres, w)
		args = append(args, a)
	}
	return wheres, args, nil
}

func containsWildcard(s string) bool {
	for _, c := range s {
		if c == '*' {
			return true
		}
	}
	return false
}

func nullInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}
