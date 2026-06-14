package handler

import (
	"net/http"

	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/internal/service/settings"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// SettingsHandler serves settings endpoints.
type SettingsHandler struct {
	settings *settings.Service
	auth     *auth.Service
}

// NewSettingsHandler returns a settings handler.
func NewSettingsHandler(settingsSvc *settings.Service, authSvc *auth.Service) *SettingsHandler {
	return &SettingsHandler{
		settings: settingsSvc,
		auth:     authSvc,
	}
}

// UpdateProfile handles PUT /settings/profile.
func (h *SettingsHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	token := auth.SessionFromRequest(r)
	userID, ok := h.auth.SessionUserID(token)
	if !ok {
		writeError(w, apperror.New(apperror.CodeSessionExpired, "session expired or invalid"))
		return
	}

	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}

	result, err := h.settings.UpdateProfile(r.Context(), settings.ProfileInput{
		ID:       userID,
		Name:     body.Name,
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"user": result})
}
