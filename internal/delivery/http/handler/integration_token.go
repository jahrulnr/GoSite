package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	pluginsvc "github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/jahrulnr/gosite/pkg/pluginperm"
)

// IntegrationTokenHandler serves integration token admin and introspection endpoints.
type IntegrationTokenHandler struct {
	tokens *pluginsvc.IntegrationTokenService
	auth   *auth.Service
	users  *sqlite.UserRepository
}

// NewIntegrationTokenHandler returns an integration token handler.
func NewIntegrationTokenHandler(tokens *pluginsvc.IntegrationTokenService, authSvc *auth.Service, users *sqlite.UserRepository) *IntegrationTokenHandler {
	return &IntegrationTokenHandler{tokens: tokens, auth: authSvc, users: users}
}

type integrationTokenJSON struct {
	ID                  string   `json:"id"`
	PluginID            string   `json:"plugin_id"`
	Label               string   `json:"label"`
	Scopes              []string `json:"scopes"`
	CreatedUnderVersion string   `json:"created_under_version,omitempty"`
	CreatedAt           string   `json:"created_at"`
	ExpiresAt           *string  `json:"expires_at,omitempty"`
	RevokedAt           *string  `json:"revoked_at,omitempty"`
	LastUsedAt          *string  `json:"last_used_at,omitempty"`
}

// Create handles POST /plugins/{vendor}/{name}/integration-tokens.
func (h *IntegrationTokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	userID, email, err := h.sessionActor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Label     string   `json:"label"`
		Scopes    []string `json:"scopes"`
		ExpiresAt *string  `json:"expires_at"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	var expiresAt *time.Time
	if body.ExpiresAt != nil && strings.TrimSpace(*body.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*body.ExpiresAt))
		if err != nil {
			writeError(w, apperror.New(apperror.CodeInvalidInput, "invalid expires_at"))
			return
		}
		expiresAt = &parsed
	}
	result, err := h.tokens.Create(r.Context(), pluginID, userID, pluginsvc.CreateTokenInput{
		Label:     body.Label,
		Scopes:    body.Scopes,
		ExpiresAt: expiresAt,
	}, email)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token": integrationTokenDTO(result.Token, result.Scopes),
		"secret": map[string]string{
			"token": result.Plaintext,
		},
	})
}

// List handles GET /plugins/{vendor}/{name}/integration-tokens.
func (h *IntegrationTokenHandler) List(w http.ResponseWriter, r *http.Request) {
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	rows, err := h.tokens.List(r.Context(), pluginID)
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]integrationTokenJSON, 0, len(rows))
	for _, row := range rows {
		scopes, _ := pluginsvc.DecodeTokenScopes(row.ScopesJSON)
		out = append(out, integrationTokenDTO(row, scopes))
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": out})
}

// Patch handles PATCH /plugins/{vendor}/{name}/integration-tokens/{tokenId}.
func (h *IntegrationTokenHandler) Patch(w http.ResponseWriter, r *http.Request) {
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	tokenID, err := integrationTokenIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	_, email, err := h.sessionActor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var body struct {
		Scopes []string `json:"scopes"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}
	updated, scopes, err := h.tokens.UpdateScopes(r.Context(), pluginID, tokenID, body.Scopes, email)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": integrationTokenDTO(updated, scopes)})
}

// Delete handles DELETE /plugins/{vendor}/{name}/integration-tokens/{tokenId}.
func (h *IntegrationTokenHandler) Delete(w http.ResponseWriter, r *http.Request) {
	pluginID, err := pluginIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	tokenID, err := integrationTokenIDFromPath(r)
	if err != nil {
		writeError(w, err)
		return
	}
	_, email, err := h.sessionActor(r)
	if err != nil {
		writeError(w, err)
		return
	}
	revoked, err := h.tokens.Revoke(r.Context(), pluginID, tokenID, email)
	if err != nil {
		writeError(w, err)
		return
	}
	scopes, _ := pluginsvc.DecodeTokenScopes(revoked.ScopesJSON)
	writeJSON(w, http.StatusOK, map[string]any{"token": integrationTokenDTO(revoked, scopes)})
}

// Self handles GET /integration-tokens/self.
func (h *IntegrationTokenHandler) Self(w http.ResponseWriter, r *http.Request) {
	plaintext := strings.TrimSpace(r.Header.Get("X-Gosite-Access-Token"))
	row, scopes, err := h.tokens.Introspect(r.Context(), plaintext)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"plugin_id":  row.PluginID,
		"label":      row.Label,
		"scopes":     scopes,
		"expires_at": formatOptionalTime(row.ExpiresAt),
	})
}

// Registry handles GET /plugins/permissions/registry.
func (h *IntegrationTokenHandler) Registry(w http.ResponseWriter, r *http.Request) {
	scopes := pluginperm.All()
	out := make([]map[string]string, 0, len(scopes))
	for _, scope := range scopes {
		out = append(out, map[string]string{"scope": scope})
	}
	writeJSON(w, http.StatusOK, map[string]any{"scopes": out})
}

func (h *IntegrationTokenHandler) sessionActor(r *http.Request) (int64, string, error) {
	token := auth.SessionFromRequest(r)
	if token == "" {
		return 0, "", apperror.New(apperror.CodeUnauthorized, "authentication required")
	}
	userID, ok := h.auth.SessionUserID(token)
	if !ok {
		return 0, "", apperror.New(apperror.CodeSessionExpired, "session expired or invalid")
	}
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil {
		return 0, "", apperror.Wrap(apperror.CodeDatabase, "load user failed", err)
	}
	return userID, user.Email, nil
}

func integrationTokenIDFromPath(r *http.Request) (string, error) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, part := range parts {
		if part == "integration-tokens" && i+1 < len(parts) {
			id := strings.TrimSpace(parts[i+1])
			if id != "" {
				return id, nil
			}
		}
	}
	return "", apperror.New(apperror.CodeInvalidInput, "token id required")
}

func integrationTokenDTO(row sqlite.PluginAccessToken, scopes []string) integrationTokenJSON {
	return integrationTokenJSON{
		ID:                  row.ID,
		PluginID:            row.PluginID,
		Label:               row.Label,
		Scopes:              scopes,
		CreatedUnderVersion: row.CreatedUnderVersion,
		CreatedAt:           row.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt:           formatOptionalTime(row.ExpiresAt),
		RevokedAt:           formatOptionalTime(row.RevokedAt),
		LastUsedAt:          formatOptionalTime(row.LastUsedAt),
	}
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(r.RemoteAddr)
}

// ManifestPermissionsFromEnabled decodes manifest permissions for UI ceiling.
func ManifestPermissionsFromEnabled(manifestJSON string) []string {
	var payload struct {
		Permissions []string `json:"permissions"`
	}
	_ = json.Unmarshal([]byte(manifestJSON), &payload)
	return payload.Permissions
}
