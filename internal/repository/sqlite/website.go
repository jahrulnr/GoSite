package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	WebsiteTypeStatic = "static"
	WebsiteTypeProxy  = "proxy"
)

// Website is a site record with nginx metadata.
type Website struct {
	ID        int64
	Name      string
	Domain    string
	Path      string
	Type      string
	Upstream  string
	SSL       bool
	Config    string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WebsiteRepository persists website records.
type WebsiteRepository struct {
	db *sql.DB
}

// NewWebsiteRepository returns a website repository backed by db.
func NewWebsiteRepository(db *sql.DB) *WebsiteRepository {
	return &WebsiteRepository{db: db}
}

// Create inserts a website and returns the stored row.
func (r *WebsiteRepository) Create(ctx context.Context, site Website) (Website, error) {
	now := time.Now().UTC()
	if site.Type == "" {
		site.Type = WebsiteTypeStatic
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO websites (name, domain, path, type, upstream, ssl, config, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, site.Name, site.Domain, site.Path, site.Type, site.Upstream, boolToInt(site.SSL), site.Config, boolToInt(site.Active), now, now)
	if err != nil {
		return Website{}, fmt.Errorf("insert website: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Website{}, fmt.Errorf("last insert id: %w", err)
	}
	return r.FindByID(ctx, id)
}

// FindByID loads a website by primary key.
func (r *WebsiteRepository) FindByID(ctx context.Context, id int64) (Website, error) {
	var site Website
	var ssl, active int
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, domain, path, type, upstream, ssl, config, active, created_at, updated_at
		FROM websites WHERE id = ?
	`, id).Scan(&site.ID, &site.Name, &site.Domain, &site.Path, &site.Type, &site.Upstream,
		&ssl, &site.Config, &active, &createdAt, &updatedAt)
	if err != nil {
		return Website{}, fmt.Errorf("find website by id: %w", err)
	}
	site.SSL = ssl == 1
	site.Active = active == 1
	if createdAt.Valid {
		site.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		site.UpdatedAt = updatedAt.Time
	}
	return site, nil
}

// FindByDomain loads a website by domain name.
func (r *WebsiteRepository) FindByDomain(ctx context.Context, domain string) (Website, error) {
	var site Website
	var ssl, active int
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, domain, path, type, upstream, ssl, config, active, created_at, updated_at
		FROM websites WHERE domain = ?
	`, domain).Scan(&site.ID, &site.Name, &site.Domain, &site.Path, &site.Type, &site.Upstream,
		&ssl, &site.Config, &active, &createdAt, &updatedAt)
	if err != nil {
		return Website{}, fmt.Errorf("find website by domain: %w", err)
	}
	site.SSL = ssl == 1
	site.Active = active == 1
	if createdAt.Valid {
		site.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		site.UpdatedAt = updatedAt.Time
	}
	return site, nil
}

// List returns all websites ordered by id.
func (r *WebsiteRepository) List(ctx context.Context) ([]Website, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, domain, path, type, upstream, ssl, config, active, created_at, updated_at
		FROM websites ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list websites: %w", err)
	}
	defer rows.Close()

	var sites []Website
	for rows.Next() {
		var site Website
		var ssl, active int
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(&site.ID, &site.Name, &site.Domain, &site.Path, &site.Type, &site.Upstream,
			&ssl, &site.Config, &active, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan website: %w", err)
		}
		site.SSL = ssl == 1
		site.Active = active == 1
		if createdAt.Valid {
			site.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			site.UpdatedAt = updatedAt.Time
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

// Update replaces mutable website fields.
func (r *WebsiteRepository) Update(ctx context.Context, site Website) (Website, error) {
	now := time.Now().UTC()
	if site.Type == "" {
		site.Type = WebsiteTypeStatic
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE websites
		SET name = ?, domain = ?, path = ?, type = ?, upstream = ?, ssl = ?, config = ?, active = ?, updated_at = ?
		WHERE id = ?
	`, site.Name, site.Domain, site.Path, site.Type, site.Upstream, boolToInt(site.SSL), site.Config,
		boolToInt(site.Active), now, site.ID)
	if err != nil {
		return Website{}, fmt.Errorf("update website: %w", err)
	}
	return r.FindByID(ctx, site.ID)
}

// Delete removes a website row by id.
func (r *WebsiteRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM websites WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete website: %w", err)
	}
	return nil
}

// ExistsPathForOther returns true when path is used by another website.
func (r *WebsiteRepository) ExistsPathForOther(ctx context.Context, path string, excludeID int64) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM websites WHERE path = ? AND id != ?
	`, path, excludeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check path duplicate: %w", err)
	}
	return count > 0, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
