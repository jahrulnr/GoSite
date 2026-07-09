package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PluginAccessToken is one integration access token row.
type PluginAccessToken struct {
	ID                  string
	PluginID            string
	CreatedUnderVersion string
	Label               string
	TokenHash           string
	ScopesJSON          string
	CreatedByUserID     int64
	CreatedAt           time.Time
	ExpiresAt           *time.Time
	RevokedAt           *time.Time
	LastUsedAt          *time.Time
}

// PluginAccessTokenRepository persists integration tokens.
type PluginAccessTokenRepository struct {
	db *sql.DB
}

// NewPluginAccessTokenRepository returns a token repository.
func NewPluginAccessTokenRepository(db *sql.DB) *PluginAccessTokenRepository {
	return &PluginAccessTokenRepository{db: db}
}

// Create inserts a new token row.
func (r *PluginAccessTokenRepository) Create(ctx context.Context, token PluginAccessToken) (PluginAccessToken, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO plugin_access_tokens (
			id, plugin_id, created_under_version, label, token_hash, scopes_json,
			created_by_user_id, created_at, expires_at, revoked_at, last_used_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, token.ID, token.PluginID, token.CreatedUnderVersion, token.Label, token.TokenHash, token.ScopesJSON,
		token.CreatedByUserID, now, nullTime(token.ExpiresAt), nullTime(token.RevokedAt), nullTime(token.LastUsedAt))
	if err != nil {
		return PluginAccessToken{}, fmt.Errorf("insert access token: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return PluginAccessToken{}, errors.New("insert access token: no row")
	}
	token.CreatedAt = now
	return token, nil
}

// ListByPlugin returns token metadata for a plugin (includes revoked).
func (r *PluginAccessTokenRepository) ListByPlugin(ctx context.Context, pluginID string) ([]PluginAccessToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plugin_id, created_under_version, label, token_hash, scopes_json,
			created_by_user_id, created_at, expires_at, revoked_at, last_used_at
		FROM plugin_access_tokens
		WHERE plugin_id = ?
		ORDER BY created_at DESC
	`, pluginID)
	if err != nil {
		return nil, fmt.Errorf("list access tokens: %w", err)
	}
	defer rows.Close()
	return scanAccessTokens(rows)
}

// FindByID returns a token by id.
func (r *PluginAccessTokenRepository) FindByID(ctx context.Context, id string) (PluginAccessToken, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, plugin_id, created_under_version, label, token_hash, scopes_json,
			created_by_user_id, created_at, expires_at, revoked_at, last_used_at
		FROM plugin_access_tokens WHERE id = ?
	`, id)
	return scanAccessToken(row)
}

// FindByHash returns a token by SHA-256 hash of the plaintext secret.
func (r *PluginAccessTokenRepository) FindByHash(ctx context.Context, hash string) (PluginAccessToken, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, plugin_id, created_under_version, label, token_hash, scopes_json,
			created_by_user_id, created_at, expires_at, revoked_at, last_used_at
		FROM plugin_access_tokens WHERE token_hash = ?
	`, hash)
	return scanAccessToken(row)
}

// UpdateScopes replaces scopes_json for a token.
func (r *PluginAccessTokenRepository) UpdateScopes(ctx context.Context, id, scopesJSON string) (PluginAccessToken, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE plugin_access_tokens SET scopes_json = ? WHERE id = ? AND revoked_at IS NULL
	`, scopesJSON, id)
	if err != nil {
		return PluginAccessToken{}, fmt.Errorf("update token scopes: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return PluginAccessToken{}, sql.ErrNoRows
	}
	return r.FindByID(ctx, id)
}

// Revoke sets revoked_at for a token.
func (r *PluginAccessTokenRepository) Revoke(ctx context.Context, id string, at time.Time) (PluginAccessToken, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE plugin_access_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL
	`, at.UTC(), id)
	if err != nil {
		return PluginAccessToken{}, fmt.Errorf("revoke token: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return PluginAccessToken{}, sql.ErrNoRows
	}
	return r.FindByID(ctx, id)
}

// RevokeAllForPlugin hard-revokes every non-revoked token for plugin_id.
func (r *PluginAccessTokenRepository) RevokeAllForPlugin(ctx context.Context, pluginID string, at time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE plugin_access_tokens SET revoked_at = ?
		WHERE plugin_id = ? AND revoked_at IS NULL
	`, at.UTC(), pluginID)
	if err != nil {
		return 0, fmt.Errorf("revoke plugin tokens: %w", err)
	}
	return res.RowsAffected()
}

// ListActiveByPlugin returns non-revoked tokens for reconciliation.
func (r *PluginAccessTokenRepository) ListActiveByPlugin(ctx context.Context, pluginID string) ([]PluginAccessToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plugin_id, created_under_version, label, token_hash, scopes_json,
			created_by_user_id, created_at, expires_at, revoked_at, last_used_at
		FROM plugin_access_tokens
		WHERE plugin_id = ? AND revoked_at IS NULL
		ORDER BY created_at ASC
	`, pluginID)
	if err != nil {
		return nil, fmt.Errorf("list active tokens: %w", err)
	}
	defer rows.Close()
	return scanAccessTokens(rows)
}

// TouchLastUsed updates last_used_at.
func (r *PluginAccessTokenRepository) TouchLastUsed(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE plugin_access_tokens SET last_used_at = ? WHERE id = ?
	`, at.UTC(), id)
	if err != nil {
		return fmt.Errorf("touch last_used_at: %w", err)
	}
	return nil
}

func scanAccessTokens(rows *sql.Rows) ([]PluginAccessToken, error) {
	var out []PluginAccessToken
	for rows.Next() {
		item, err := scanAccessTokenRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanAccessToken(row *sql.Row) (PluginAccessToken, error) {
	return scanAccessTokenRow(row)
}

type accessTokenScanner interface {
	Scan(dest ...any) error
}

func scanAccessTokenRow(row accessTokenScanner) (PluginAccessToken, error) {
	var token PluginAccessToken
	var expiresAt, revokedAt, lastUsedAt sql.NullTime
	err := row.Scan(
		&token.ID, &token.PluginID, &token.CreatedUnderVersion, &token.Label, &token.TokenHash, &token.ScopesJSON,
		&token.CreatedByUserID, &token.CreatedAt, &expiresAt, &revokedAt, &lastUsedAt,
	)
	if err != nil {
		return PluginAccessToken{}, err
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		token.ExpiresAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		token.RevokedAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		token.LastUsedAt = &t
	}
	return token, nil
}
