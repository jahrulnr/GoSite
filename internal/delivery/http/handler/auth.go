package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/delivery/http/middleware"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// AuthHandler serves panel authentication endpoints.
type AuthHandler struct {
	auth     *auth.Service
	sessions *auth.Store
	meta     auth.LoginMetadata
}

// NewAuthHandler returns an auth HTTP handler.
func NewAuthHandler(svc *auth.Service, sessions *auth.Store, meta auth.LoginMetadata) *AuthHandler {
	return &AuthHandler{
		auth:     svc,
		sessions: sessions,
		meta:     meta,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// LoginMetadata handles GET /auth/login (public panel auth metadata).
func (h *AuthHandler) LoginMetadata(c *gin.Context) {
	c.JSON(http.StatusOK, h.meta)
}

// Login authenticates a user and issues a session cookie.
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c.Writer, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}

	result, err := h.auth.Login(c.Request.Context(), req.Email, req.Password, req.Remember)
	if err != nil {
		writeError(c.Writer, err)
		return
	}

	h.sessions.SetCookie(c.Writer, c.Request, result.Session)
	c.JSON(http.StatusOK, gin.H{
		"token": result.Token,
		"user":  result.User,
	})
}

// Logout destroys the current session.
func (h *AuthHandler) Logout(c *gin.Context) {
	token := middleware.SessionToken(c)
	h.auth.Logout(token)
	h.sessions.ClearCookie(c.Writer, c.Request)
	c.Status(http.StatusNoContent)
}

// Me returns the authenticated user profile.
func (h *AuthHandler) Me(c *gin.Context) {
	token := middleware.SessionToken(c)
	user, err := h.auth.Me(c.Request.Context(), token)
	if err != nil {
		writeError(c.Writer, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

// Lockscreen handles GET /auth/lockscreen.
func (h *AuthHandler) Lockscreen(c *gin.Context) {
	token := middleware.SessionToken(c)
	status, err := h.auth.LockscreenStatus(c.Request.Context(), token)
	if err != nil {
		writeError(c.Writer, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// Lock handles POST /auth/lock (client idle timeout).
func (h *AuthHandler) Lock(c *gin.Context) {
	token := middleware.SessionToken(c)
	if err := h.auth.LockSession(token); err != nil {
		writeError(c.Writer, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"locked": true})
}

// Unlock handles POST /auth/unlock.
func (h *AuthHandler) Unlock(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c.Writer, apperror.New(apperror.CodeValidation, "invalid request body"))
		return
	}
	token := middleware.SessionToken(c)
	if err := h.auth.Unlock(c.Request.Context(), token, body.Password); err != nil {
		writeError(c.Writer, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"locked": false})
}
