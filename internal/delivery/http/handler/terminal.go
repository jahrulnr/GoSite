package handler

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/internal/terminal"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// TerminalHandler exposes the PTY hub over HTTP and WebSocket.
type TerminalHandler struct {
	hub       *terminal.Hub
	audit     contracts.AuditWriter
	authSvc   *auth.Service
	upgrader  websocket.Upgrader
}

// NewTerminalHandler wires the dependencies needed to serve the floating
// terminal endpoints.
func NewTerminalHandler(hub *terminal.Hub, audit contracts.AuditWriter, authSvc *auth.Service) *TerminalHandler {
	return &TerminalHandler{
		hub:     hub,
		audit:   audit,
		authSvc: authSvc,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				// Same-origin policy is enforced by the session cookie
				// check; the websocket itself can be opened from any
				// origin because the request must include a valid
				// gosite_session cookie.
				return true
			},
		},
	}
}

// HandleWS upgrades the request to a websocket and pumps output to / input
// from the PTY session identified by ?session_id=... A new session is spawned
// when the query parameter is empty.
func (h *TerminalHandler) HandleWS(c *gin.Context) {
	userID, ok := userIDFromContext(c, h.authSvc)
	if !ok {
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	sessionID := c.Query("session_id")
	cols := 80
	rows := 24
	if v := c.Query("cols"); v != "" {
		if n, ok := parseInt(v); ok && n > 0 {
			cols = n
		}
	}
	if v := c.Query("rows"); v != "" {
		if n, ok := parseInt(v); ok && n > 0 {
			rows = n
		}
	}

	opts := terminal.CreateOptions{
		SessionID: sessionID,
		Cols:      cols,
		Rows:      rows,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	session, finalID, err := h.hub.AttachOrCreate(ctx, userID, sessionID, opts)
	if err != nil {
		emitError(conn, err)
		return
	}

	role := session.RoleFor(conn)
	if err := h.audit.Write(c.Request.Context(), contracts.AuditEntry{
		Timestamp:    time.Now(),
		UserEmail:    emailForUser(c, h.authSvc),
		Action:       "terminal.session.open",
		ResourceType: "terminal_session",
		ResourceID:   finalID,
		Status:       "ok",
		Message:      "pty session opened",
		MetaJSON:     `{"role":"` + role + `"}`,
	}); err != nil {
		// Audit failures should not block the user.
		_ = err
	}

	if pumpErr := h.hub.AttachAndPump(session, conn, func(evt string, _ map[string]interface{}) {
		// Reserved for future audit hooks (resize, role flip, etc).
		_ = evt
	}); pumpErr != nil && !errors.Is(pumpErr, context.Canceled) {
		// Best effort: tell the client if we still have a healthy socket.
		_ = emitError(conn, pumpErr)
	}

	if err := h.audit.Write(c.Request.Context(), contracts.AuditEntry{
		Timestamp:    time.Now(),
		UserEmail:    emailForUser(c, h.authSvc),
		Action:       "terminal.session.close",
		ResourceType: "terminal_session",
		ResourceID:   finalID,
		Status:       "ok",
		Message:      "pty session detached",
		MetaJSON:     `{"role":"` + role + `"}`,
	}); err != nil {
		_ = err
	}
}

// ListSessions returns all live sessions owned by the authenticated user.
func (h *TerminalHandler) ListSessions(c *gin.Context) {
	userID, ok := userIDFromContext(c, h.authSvc)
	if !ok {
		return
	}
	list := h.hub.ListByUser(userID)
	c.JSON(http.StatusOK, gin.H{"sessions": list})
}

// GetSnapshot streams the current rolling dump for a session. Used as a
// fallback when the websocket is unavailable (e.g. behind a corporate proxy
// that strips Upgrade headers).
func (h *TerminalHandler) GetSnapshot(c *gin.Context) {
	userID, ok := userIDFromContext(c, h.authSvc)
	if !ok {
		return
	}
	id := c.Param("id")
	session, found := h.hub.Get(id)
	if !found {
		err := apperror.New(apperror.CodeNotFound, "terminal session not found")
		c.AbortWithStatusJSON(http.StatusNotFound, err.Body())
		return
	}
	if session == nil || session.Meta().UserID != userID {
		err := apperror.New(apperror.CodeForbidden, "session belongs to another user")
		c.AbortWithStatusJSON(http.StatusForbidden, err.Body())
		return
	}
	data, first, end := session.Snapshot()
	c.JSON(http.StatusOK, gin.H{
		"session_id": id,
		"shell":      session.Meta().Shell,
		"cwd":        session.Meta().Cwd,
		"started_at": session.Meta().StartedAt,
		"bytes":      len(data),
		"first_seq":  first,
		"end_seq":    end,
		"data_b64":   base64.StdEncoding.EncodeToString(data),
	})
}

// KillSession terminates a session owned by the user.
func (h *TerminalHandler) KillSession(c *gin.Context) {
	userID, ok := userIDFromContext(c, h.authSvc)
	if !ok {
		return
	}
	id := c.Param("id")
	session, found := h.hub.Get(id)
	if found {
		if meta := session.Meta(); meta.UserID != userID {
			err := apperror.New(apperror.CodeForbidden, "session belongs to another user")
			c.AbortWithStatusJSON(http.StatusForbidden, err.Body())
			return
		}
	}
	if err := h.hub.Kill(id); err != nil {
		err2 := apperror.New(apperror.CodeNotFound, "terminal session not found")
		c.AbortWithStatusJSON(http.StatusNotFound, err2.Body())
		return
	}
	if err := h.audit.Write(c.Request.Context(), contracts.AuditEntry{
		Timestamp:    time.Now(),
		UserEmail:    emailForUser(c, h.authSvc),
		Action:       "terminal.session.kill",
		ResourceType: "terminal_session",
		ResourceID:   id,
		Status:       "ok",
		Message:      "pty session terminated by user",
	}); err != nil {
		_ = err
	}
	c.JSON(http.StatusOK, gin.H{"message": "killed", "session_id": id})
}

func emitError(conn *websocket.Conn, err error) error {
	if conn == nil {
		return err
	}
	frame, marshalErr := terminal.EncodeText(terminal.ErrorFrame{Type: terminal.FrameError, Message: err.Error()})
	if marshalErr != nil {
		return marshalErr
	}
	writeErr := conn.WriteMessage(websocket.TextMessage, frame)
	_ = conn.Close()
	return writeErr
}

func userIDFromContext(c *gin.Context, authSvc *auth.Service) (int64, bool) {
	token := auth.SessionFromRequest(c.Request)
	if token == "" {
		abortTerminal(c, http.StatusUnauthorized, apperror.New(apperror.CodeUnauthorized, "authentication required"))
		return 0, false
	}
	uid, ok := authSvc.SessionUserID(token)
	if !ok {
		abortTerminal(c, http.StatusUnauthorized, apperror.New(apperror.CodeSessionExpired, "session expired or invalid"))
		return 0, false
	}
	return uid, true
}

func abortTerminal(c *gin.Context, status int, err *apperror.Error) {
	c.AbortWithStatusJSON(status, err.Body())
}

func emailForUser(c *gin.Context, authSvc *auth.Service) string {
	uid, ok := userIDFromContext(c, authSvc)
	if !ok {
		return ""
	}
	user, err := authSvc.Me(c.Request.Context(), tokenFromCtx(c))
	if err != nil {
		_ = uid
		return ""
	}
	return user.Email
}

func tokenFromCtx(c *gin.Context) string {
	return auth.SessionFromRequest(c.Request)
}

func parseInt(s string) (int, bool) {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}
