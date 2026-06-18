package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// AuditLog is a persisted audit event.
type AuditLog struct {
	ID           int64
	Timestamp    time.Time
	UserEmail    string
	Action       string
	ResourceType string
	ResourceID   string
	Domain       string
	Status       string
	Message      string
	MetaJSON     string
}

// AuditRepository reads and writes audit_logs rows.
type AuditRepository struct {
	db *sql.DB
}

// NewAuditRepository returns an audit repository backed by db.
func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Write inserts a new audit log entry.
func (r *AuditRepository) Write(ctx context.Context, entry AuditLog) error {
	ts := entry.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	meta := entry.MetaJSON
	if meta == "" {
		meta = "{}"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (ts, user_email, action, resource_type, resource_id, domain, status, message, meta_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ts, entry.UserEmail, entry.Action, entry.ResourceType, entry.ResourceID, entry.Domain, entry.Status, entry.Message, meta)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// AuditFilter constrains audit log queries.
type AuditFilter struct {
	From   *time.Time
	To     *time.Time
	Wheres []string
	Args   []interface{}
	Limit  int
	Offset int
}

// List returns audit rows matching filter ordered by ts desc.
func (r *AuditRepository) List(ctx context.Context, f AuditFilter) ([]AuditLog, error) {
	query := `
		SELECT id, ts, user_email, action, resource_type, resource_id, domain, status, message, meta_json
		FROM audit_logs WHERE 1=1`
	args := make([]interface{}, 0, len(f.Args)+4)
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
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var out []AuditLog
	for rows.Next() {
		var entry AuditLog
		var ts sql.NullTime
		if err := rows.Scan(
			&entry.ID, &ts, &entry.UserEmail, &entry.Action, &entry.ResourceType,
			&entry.ResourceID, &entry.Domain, &entry.Status, &entry.Message, &entry.MetaJSON,
		); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		if ts.Valid {
			entry.Timestamp = ts.Time
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

// Count returns matching audit rows.
func (r *AuditRepository) Count(ctx context.Context, f AuditFilter) (int, error) {
	query := `SELECT COUNT(1) FROM audit_logs WHERE 1=1`
	args := make([]interface{}, 0, len(f.Args)+2)
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
		return 0, fmt.Errorf("count audit logs: %w", err)
	}
	return count, nil
}

// Recent returns the newest audit entries up to limit.
func (r *AuditRepository) Recent(ctx context.Context, limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, ts, user_email, action, resource_type, resource_id, domain, status, message, meta_json
		FROM audit_logs
		ORDER BY ts DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var out []AuditLog
	for rows.Next() {
		var entry AuditLog
		var ts sql.NullTime
		if err := rows.Scan(
			&entry.ID, &ts, &entry.UserEmail, &entry.Action, &entry.ResourceType,
			&entry.ResourceID, &entry.Domain, &entry.Status, &entry.Message, &entry.MetaJSON,
		); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		if ts.Valid {
			entry.Timestamp = ts.Time
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}
	return out, nil
}

// PurgeOlderThan deletes audit rows before cutoff.
func (r *AuditRepository) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM audit_logs WHERE ts < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge audit logs: %w", err)
	}
	return res.RowsAffected()
}

// ListSince returns audit rows with ts > since, ordered by ts asc.
func (r *AuditRepository) ListSince(ctx context.Context, since time.Time) ([]AuditLog, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, ts, user_email, action, resource_type, resource_id, domain, status, message, meta_json
		FROM audit_logs
		WHERE ts > ?
		ORDER BY ts ASC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("list audit since: %w", err)
	}
	defer rows.Close()

	var out []AuditLog
	for rows.Next() {
		var entry AuditLog
		var ts sql.NullTime
		if err := rows.Scan(
			&entry.ID, &ts, &entry.UserEmail, &entry.Action, &entry.ResourceType,
			&entry.ResourceID, &entry.Domain, &entry.Status, &entry.Message, &entry.MetaJSON,
		); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		if ts.Valid {
			entry.Timestamp = ts.Time
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}
