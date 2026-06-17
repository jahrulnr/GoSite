package plugin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/jahrulnr/gosite/pkg/pluginperm"
)

const accessTokenPrefix = "gs_pat_"

// IntegrationTokenService manages scoped plugin access tokens.
type IntegrationTokenService struct {
	tokens  *sqlite.PluginAccessTokenRepository
	plugins *sqlite.PluginRepository
	audit   contracts.AuditWriter

	usedMu    sync.Mutex
	usedDedup map[string]time.Time
}

// NewIntegrationTokenService returns an integration token service.
func NewIntegrationTokenService(tokens *sqlite.PluginAccessTokenRepository, plugins *sqlite.PluginRepository, audit contracts.AuditWriter) *IntegrationTokenService {
	return &IntegrationTokenService{
		tokens:    tokens,
		plugins:   plugins,
		audit:     audit,
		usedDedup: make(map[string]time.Time),
	}
}

// CreateTokenInput is the admin create request.
type CreateTokenInput struct {
	Label     string
	Scopes    []string
	ExpiresAt *time.Time
}

// CreateTokenResult includes the one-time plaintext secret.
type CreateTokenResult struct {
	Token     sqlite.PluginAccessToken
	Plaintext string
	Scopes    []string
}

// Create issues a new access token after validating scopes against the enabled manifest.
func (s *IntegrationTokenService) Create(ctx context.Context, pluginID string, userID int64, input CreateTokenInput, actorEmail string) (CreateTokenResult, error) {
	enabled, manifest, err := s.enabledManifest(ctx, pluginID)
	if err != nil {
		return CreateTokenResult{}, err
	}
	scopes, err := normalizeScopes(input.Scopes)
	if err != nil {
		return CreateTokenResult{}, err
	}
	if !pluginperm.SubsetOfManifest(scopes, manifest.Permissions) {
		return CreateTokenResult{}, apperror.New(apperror.CodeValidation, "scopes exceed manifest permissions")
	}
	plaintext, hash, err := generateAccessTokenSecret()
	if err != nil {
		return CreateTokenResult{}, apperror.Wrap(apperror.CodeInternal, "generate token failed", err)
	}
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return CreateTokenResult{}, apperror.Wrap(apperror.CodeInternal, "encode scopes failed", err)
	}
	row, err := s.tokens.Create(ctx, sqlite.PluginAccessToken{
		ID:                  uuid.NewString(),
		PluginID:            pluginID,
		CreatedUnderVersion: enabled.Version,
		Label:               strings.TrimSpace(input.Label),
		TokenHash:           hash,
		ScopesJSON:          string(scopesJSON),
		CreatedByUserID:     userID,
		ExpiresAt:           input.ExpiresAt,
	})
	if err != nil {
		return CreateTokenResult{}, apperror.Wrap(apperror.CodeDatabase, "create token failed", err)
	}
	s.auditEvent(ctx, actorEmail, "integration_token.created", row.ID, pluginID, "ok", map[string]any{
		"token_id": row.ID,
		"label":    row.Label,
		"scopes":   scopes,
	})
	return CreateTokenResult{Token: row, Plaintext: plaintext, Scopes: scopes}, nil
}

// List returns token metadata for a plugin.
func (s *IntegrationTokenService) List(ctx context.Context, pluginID string) ([]sqlite.PluginAccessToken, error) {
	if _, err := s.plugins.FindEnabled(ctx, pluginID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Allow listing when plugin exists but disabled — tokens may still be suspended.
			if _, findErr := s.anyPluginVersion(ctx, pluginID); findErr != nil {
				return nil, apperror.New(apperror.CodeNotFound, "plugin not found")
			}
		} else {
			return nil, apperror.Wrap(apperror.CodeDatabase, "find plugin failed", err)
		}
	}
	rows, err := s.tokens.ListByPlugin(ctx, pluginID)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "list tokens failed", err)
	}
	return rows, nil
}

// UpdateScopes patches the scope whitelist.
func (s *IntegrationTokenService) UpdateScopes(ctx context.Context, pluginID, tokenID string, scopes []string, actorEmail string) (sqlite.PluginAccessToken, []string, error) {
	enabled, manifest, err := s.enabledManifest(ctx, pluginID)
	if err != nil {
		return sqlite.PluginAccessToken{}, nil, err
	}
	_ = enabled
	normalized, err := normalizeScopes(scopes)
	if err != nil {
		return sqlite.PluginAccessToken{}, nil, err
	}
	if !pluginperm.SubsetOfManifest(normalized, manifest.Permissions) {
		return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeValidation, "scopes exceed manifest permissions")
	}
	row, err := s.tokens.FindByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeNotFound, "token not found")
		}
		return sqlite.PluginAccessToken{}, nil, apperror.Wrap(apperror.CodeDatabase, "find token failed", err)
	}
	if row.PluginID != pluginID {
		return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeNotFound, "token not found")
	}
	if row.RevokedAt != nil {
		return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeConflict, "token revoked")
	}
	scopesJSON, err := json.Marshal(normalized)
	if err != nil {
		return sqlite.PluginAccessToken{}, nil, apperror.Wrap(apperror.CodeInternal, "encode scopes failed", err)
	}
	updated, err := s.tokens.UpdateScopes(ctx, tokenID, string(scopesJSON))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeNotFound, "token not found")
		}
		return sqlite.PluginAccessToken{}, nil, apperror.Wrap(apperror.CodeDatabase, "update scopes failed", err)
	}
	s.auditEvent(ctx, actorEmail, "integration_token.scopes_updated", tokenID, pluginID, "ok", map[string]any{
		"token_id": tokenID,
		"scopes":   normalized,
	})
	return updated, normalized, nil
}

// Revoke hard-revokes a token.
func (s *IntegrationTokenService) Revoke(ctx context.Context, pluginID, tokenID, actorEmail string) (sqlite.PluginAccessToken, error) {
	row, err := s.tokens.FindByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginAccessToken{}, apperror.New(apperror.CodeNotFound, "token not found")
		}
		return sqlite.PluginAccessToken{}, apperror.Wrap(apperror.CodeDatabase, "find token failed", err)
	}
	if row.PluginID != pluginID {
		return sqlite.PluginAccessToken{}, apperror.New(apperror.CodeNotFound, "token not found")
	}
	revoked, err := s.tokens.Revoke(ctx, tokenID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginAccessToken{}, apperror.New(apperror.CodeNotFound, "token not found")
		}
		return sqlite.PluginAccessToken{}, apperror.Wrap(apperror.CodeDatabase, "revoke token failed", err)
	}
	s.auditEvent(ctx, actorEmail, "integration_token.revoked", tokenID, pluginID, "ok", map[string]any{"token_id": tokenID})
	return revoked, nil
}

// Introspect validates a plaintext token and returns metadata for MCP self endpoint.
func (s *IntegrationTokenService) Introspect(ctx context.Context, plaintext string) (sqlite.PluginAccessToken, []string, error) {
	row, scopes, err := s.authenticatePlaintext(ctx, plaintext)
	if err != nil {
		return sqlite.PluginAccessToken{}, nil, err
	}
	return row, scopes, nil
}

// AuthenticatedToken is the validated access token with scopes.
type AuthenticatedToken struct {
	Row    sqlite.PluginAccessToken
	Scopes []string
}

// Authenticate validates plaintext and plugin enabled state.
func (s *IntegrationTokenService) Authenticate(ctx context.Context, plaintext string) (AuthenticatedToken, error) {
	row, scopes, err := s.authenticatePlaintext(ctx, plaintext)
	if err != nil {
		return AuthenticatedToken{}, err
	}
	return AuthenticatedToken{Row: row, Scopes: scopes}, nil
}

// RecordUse audits token usage with 60s dedup per token_id+route.
func (s *IntegrationTokenService) RecordUse(ctx context.Context, tokenID, route, clientIP string) {
	_ = s.tokens.TouchLastUsed(ctx, tokenID, time.Now().UTC())
	key := tokenID + "|" + route
	now := time.Now().UTC()
	s.usedMu.Lock()
	if last, ok := s.usedDedup[key]; ok && now.Sub(last) < 60*time.Second {
		s.usedMu.Unlock()
		return
	}
	s.usedDedup[key] = now
	s.usedMu.Unlock()
	s.auditEvent(ctx, "", "integration_token.used", tokenID, "", "ok", map[string]any{
		"token_id":  tokenID,
		"route":     route,
		"client_ip": clientIP,
	})
}

// ReconcileAfterSwitch truncates or revokes tokens after a successful version switch.
func (s *IntegrationTokenService) ReconcileAfterSwitch(ctx context.Context, pluginID string, manifest Manifest, actorEmail string) error {
	active, err := s.tokens.ListActiveByPlugin(ctx, pluginID)
	if err != nil {
		return err
	}
	for _, row := range active {
		scopes, err := decodeScopes(row.ScopesJSON)
		if err != nil {
			continue
		}
		if pluginperm.SubsetOfManifest(scopes, manifest.Permissions) {
			continue
		}
		truncated := pluginperm.Intersect(scopes, manifest.Permissions)
		if len(truncated) == 0 {
			if _, err := s.tokens.Revoke(ctx, row.ID, time.Now().UTC()); err != nil {
				return err
			}
			s.auditEvent(ctx, actorEmail, "integration_token.scopes_truncated", row.ID, pluginID, "ok", map[string]any{
				"token_id": row.ID,
				"before":   scopes,
				"after":    []string{},
			})
			s.auditEvent(ctx, actorEmail, "integration_token.revoked", row.ID, pluginID, "ok", map[string]any{
				"token_id": row.ID,
				"reason":   "manifest_shrink",
			})
			continue
		}
		scopesJSON, err := json.Marshal(truncated)
		if err != nil {
			return err
		}
		if _, err := s.tokens.UpdateScopes(ctx, row.ID, string(scopesJSON)); err != nil {
			return err
		}
		s.auditEvent(ctx, actorEmail, "integration_token.scopes_truncated", row.ID, pluginID, "ok", map[string]any{
			"token_id": row.ID,
			"before":   scopes,
			"after":    truncated,
		})
	}
	return nil
}

// RevokeAllForPlugin hard-revokes all tokens when plugin is fully removed.
func (s *IntegrationTokenService) RevokeAllForPlugin(ctx context.Context, pluginID, actorEmail string) error {
	n, err := s.tokens.RevokeAllForPlugin(ctx, pluginID, time.Now().UTC())
	if err != nil {
		return err
	}
	if n > 0 {
		s.auditEvent(ctx, actorEmail, "integration_token.revoked", pluginID, pluginID, "ok", map[string]any{
			"plugin_id": pluginID,
			"count":     n,
			"reason":    "plugin_uninstalled",
		})
	}
	return nil
}

func (s *IntegrationTokenService) authenticatePlaintext(ctx context.Context, plaintext string) (sqlite.PluginAccessToken, []string, error) {
	plaintext = strings.TrimSpace(plaintext)
	if !strings.HasPrefix(plaintext, accessTokenPrefix) {
		return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeUnauthorized, "invalid access token")
	}
	hash := hashAccessToken(plaintext)
	row, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeUnauthorized, "invalid access token")
		}
		return sqlite.PluginAccessToken{}, nil, apperror.Wrap(apperror.CodeDatabase, "find token failed", err)
	}
	if row.RevokedAt != nil {
		return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeUnauthorized, "token revoked")
	}
	if row.ExpiresAt != nil && time.Now().UTC().After(row.ExpiresAt.UTC()) {
		return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeUnauthorized, "token expired")
	}
	if _, err := s.plugins.FindEnabled(ctx, row.PluginID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginAccessToken{}, nil, apperror.New(apperror.CodeUnauthorized, "plugin disabled")
		}
		return sqlite.PluginAccessToken{}, nil, apperror.Wrap(apperror.CodeDatabase, "find enabled plugin failed", err)
	}
	scopes, err := decodeScopes(row.ScopesJSON)
	if err != nil {
		return sqlite.PluginAccessToken{}, nil, apperror.Wrap(apperror.CodeInternal, "decode scopes failed", err)
	}
	return row, scopes, nil
}

func (s *IntegrationTokenService) enabledManifest(ctx context.Context, pluginID string) (sqlite.PluginVersion, Manifest, error) {
	enabled, err := s.plugins.FindEnabled(ctx, pluginID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlite.PluginVersion{}, Manifest{}, apperror.New(apperror.CodeConflict, "plugin is not enabled")
		}
		return sqlite.PluginVersion{}, Manifest{}, apperror.Wrap(apperror.CodeDatabase, "find enabled plugin failed", err)
	}
	return enabled, manifestFromRecord(enabled), nil
}

func (s *IntegrationTokenService) anyPluginVersion(ctx context.Context, pluginID string) (sqlite.PluginVersion, error) {
	rows, err := s.plugins.List(ctx)
	if err != nil {
		return sqlite.PluginVersion{}, err
	}
	for _, row := range rows {
		if row.PluginID == pluginID && row.State != sqlite.PluginStateUninstalled {
			return row, nil
		}
	}
	return sqlite.PluginVersion{}, sql.ErrNoRows
}

func (s *IntegrationTokenService) auditEvent(ctx context.Context, actorEmail, action, resourceID, pluginID, status string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	raw, _ := json.Marshal(meta)
	_ = s.audit.Write(ctx, contracts.AuditEntry{
		Timestamp:    time.Now().UTC(),
		UserEmail:    actorEmail,
		Action:       action,
		ResourceType: "integration_token",
		ResourceID:   resourceID,
		Domain:       pluginID,
		Status:       status,
		MetaJSON:     string(raw),
	})
}

func generateAccessTokenSecret() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	plaintext = accessTokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	hash = hashAccessToken(plaintext)
	return plaintext, hash, nil
}

func hashAccessToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func normalizeScopes(scopes []string) ([]string, error) {
	if len(scopes) == 0 {
		return nil, apperror.New(apperror.CodeValidation, "at least one scope required")
	}
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if !pluginperm.Valid(scope) {
			return nil, apperror.New(apperror.CodeValidation, "unknown scope: "+scope)
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	if len(out) == 0 {
		return nil, apperror.New(apperror.CodeValidation, "at least one scope required")
	}
	return out, nil
}

func decodeScopes(raw string) ([]string, error) {
	var scopes []string
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	if err := json.Unmarshal([]byte(raw), &scopes); err != nil {
		return nil, err
	}
	return scopes, nil
}

// DecodeTokenScopes exports scope decoding for handlers.
func DecodeTokenScopes(raw string) ([]string, error) {
	return decodeScopes(raw)
}
