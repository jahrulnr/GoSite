package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

const basicAuthRealm = "Access denied"

// BasicAuth gates requests when AUTH_ENABLE is true.
//
// Browser EventSource does not support custom headers (notably Authorization),
// so when BasicAuth is enabled, anything that would be consumed as a server
// stream (SSE / NDJSON) needs to bypass this check and rely on the session
// cookie instead. Without this, the Logs Live Tail stream gets stuck at 401
// and the UI shows "Failed to fetch" with no useful error.
func BasicAuth(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.AuthEnable {
			c.Next()
			return
		}

		if isStreamRequest(c) {
			c.Next()
			return
		}

		user, pass, ok := c.Request.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(cfg.AuthUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.AuthPass)) != 1 {
			c.Header("Cache-Control", "no-cache, must-revalidate, max-age=0")
			c.Header("WWW-Authenticate", `Basic realm="`+basicAuthRealm+`"`)
			err := apperror.New(apperror.CodeBasicAuthRequired, "basic authentication required")
			c.AbortWithStatusJSON(http.StatusUnauthorized, err.Body())
			return
		}

		c.Next()
	}
}

func isStreamRequest(c *gin.Context) bool {
	// WebSocket upgrades carry an Upgrade header. Browser WebSockets
	// cannot set custom headers (notably Authorization) so the basic-auth
	// gate has to be skipped and the session cookie must do the auth.
	if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
		return true
	}
	if strings.HasPrefix(c.Request.URL.Path, "/api/v1/query/tail") {
		return true
	}
	if strings.EqualFold(c.Request.URL.Query().Get("stream"), "sse") ||
		strings.EqualFold(c.Request.URL.Query().Get("stream"), "ndjson") {
		return true
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	if strings.Contains(accept, "text/event-stream") ||
		strings.Contains(accept, "application/x-ndjson") {
		return true
	}
	return false
}
