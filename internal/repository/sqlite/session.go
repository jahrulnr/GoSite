package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SessionRecord is the persisted shape of an active panel session.
type SessionRecord struct {
	ID        string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionRepository persists panel sessions in SQLite. Expired rows are pruned
// on every load so reads never serve a stale id.
type SessionRepository struct {
	db *sql.DB
}

// NewSessionRepository returns a session repository backed by db.
func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create inserts a session row.
func (r *SessionRepository) Create(ctx context.Context, rec SessionRecord) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)
	`, rec.ID, rec.UserID, rec.CreatedAt.UTC(), rec.ExpiresAt.UTC())
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// Get returns a non-expired session by id. Expired rows are deleted on access.
func (r *SessionRepository) Get(ctx context.Context, id string) (SessionRecord, bool) {
	if id == "" {
		return SessionRecord{}, false
	}
	row := r.db.QueryRowContext(ctx, `SELECT id, user_id, created_at, expires_at FROM sessions WHERE id = ?`, id)
	var rec SessionRecord
	if err := row.Scan(&rec.ID, &rec.UserID, &rec.CreatedAt, &rec.ExpiresAt); err != nil {
		if err != sql.ErrNoRows {
			return SessionRecord{}, false
		}
		return SessionRecord{}, false
	}
	if time.Now().UTC().After(rec.ExpiresAt.UTC()) {
		_ = r.Delete(ctx, id)
		return SessionRecord{}, false
	}
	return rec, true
}

// Delete removes a session by id.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// PurgeExpired removes every expired row. Safe to call on a schedule.
func (r *SessionRepository) PurgeExpired(ctx context.Context) (int, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("purge expired sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
