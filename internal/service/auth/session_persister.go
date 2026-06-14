package auth

import (
	"context"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

// SQLitePersister adapts the SQLite session repository to the auth.Persister
// interface so the in-memory session cache can fall through to persistent
// storage on cache miss.
type SQLitePersister struct {
	repo *sqlite.SessionRepository
}

// NewSQLitePersister wraps the given repository.
func NewSQLitePersister(repo *sqlite.SessionRepository) *SQLitePersister {
	return &SQLitePersister{repo: repo}
}

// Create inserts a session record.
func (p *SQLitePersister) Create(ctx context.Context, rec SessionRecord) error {
	return p.repo.Create(ctx, sqlite.SessionRecord{
		ID:        rec.ID,
		UserID:    rec.UserID,
		CreatedAt: rec.CreatedAt,
		ExpiresAt: rec.ExpiresAt,
	})
}

// Get returns a non-expired record by id.
func (p *SQLitePersister) Get(ctx context.Context, id string) (SessionRecord, bool) {
	rec, ok := p.repo.Get(ctx, id)
	if !ok {
		return SessionRecord{}, false
	}
	return SessionRecord{
		ID:        rec.ID,
		UserID:    rec.UserID,
		CreatedAt: rec.CreatedAt,
		ExpiresAt: rec.ExpiresAt,
	}, true
}

// Delete removes a session record.
func (p *SQLitePersister) Delete(ctx context.Context, id string) error {
	return p.repo.Delete(ctx, id)
}
