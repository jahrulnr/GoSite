package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// SavedQuery is a stored Splunk Lite query.
type SavedQuery struct {
	ID        int64
	Name      string
	Source    string
	Query     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SavedQueryRepository persists saved query rows.
type SavedQueryRepository struct {
	db *sql.DB
}

// NewSavedQueryRepository returns a saved query repository backed by db.
func NewSavedQueryRepository(db *sql.DB) *SavedQueryRepository {
	return &SavedQueryRepository{db: db}
}

// List returns all saved queries ordered by name.
func (r *SavedQueryRepository) List(ctx context.Context) ([]SavedQuery, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, source, query, created_at, updated_at
		FROM saved_queries ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list saved queries: %w", err)
	}
	defer rows.Close()

	var out []SavedQuery
	for rows.Next() {
		var q SavedQuery
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&q.ID, &q.Name, &q.Source, &q.Query, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan saved query: %w", err)
		}
		if createdAt.Valid {
			q.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			q.UpdatedAt = updatedAt.Time
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// Create inserts a saved query and returns the stored row.
func (r *SavedQueryRepository) Create(ctx context.Context, name, source, query string) (SavedQuery, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO saved_queries (name, source, query, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, name, source, query, now, now)
	if err != nil {
		return SavedQuery{}, fmt.Errorf("insert saved query: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return SavedQuery{}, fmt.Errorf("last insert id: %w", err)
	}
	return SavedQuery{
		ID:        id,
		Name:      name,
		Source:    source,
		Query:     query,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// FindByID loads a saved query by primary key.
func (r *SavedQueryRepository) FindByID(ctx context.Context, id int64) (SavedQuery, error) {
	var q SavedQuery
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, source, query, created_at, updated_at
		FROM saved_queries WHERE id = ?
	`, id).Scan(&q.ID, &q.Name, &q.Source, &q.Query, &createdAt, &updatedAt)
	if err != nil {
		return SavedQuery{}, fmt.Errorf("find saved query: %w", err)
	}
	if createdAt.Valid {
		q.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		q.UpdatedAt = updatedAt.Time
	}
	return q, nil
}

// Update partially updates a saved query. Empty fields are skipped.
func (r *SavedQueryRepository) Update(ctx context.Context, id int64, name, source, query string) (SavedQuery, error) {
	now := time.Now().UTC()
	sets := []string{}
	args := []interface{}{}
	if name != "" {
		sets = append(sets, "name = ?")
		args = append(args, name)
	}
	if source != "" {
		sets = append(sets, "source = ?")
		args = append(args, source)
	}
	if query != "" {
		sets = append(sets, "query = ?")
		args = append(args, query)
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, now)
	args = append(args, id)

	if len(sets) == 1 {
		// only updated_at — nothing meaningful to change; return current row
		return r.FindByID(ctx, id)
	}

	stmt := "UPDATE saved_queries SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	if _, err := r.db.ExecContext(ctx, stmt, args...); err != nil {
		return SavedQuery{}, fmt.Errorf("update saved query: %w", err)
	}
	return r.FindByID(ctx, id)
}

// Delete removes a saved query by primary key.
func (r *SavedQueryRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM saved_queries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete saved query: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("saved query not found")
	}
	return nil
}
