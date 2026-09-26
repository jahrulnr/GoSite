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
// Browser EventSource does not support custom headers (notably Authorization)
// and WebSocket handshakes carry no Authorization header either, so the
// streaming endpoints consumed by the UI rely on the session cookie instead
// of BasicAuth. The bypass is limited to the exact GET paths listed in
// streamPaths/streamPathSuffixes: deciding it from request headers such as
// Accept, Upgrade or ?stream= would let any endpoint — including
// /auth/login — slip past the BasicAuth gate entirely.
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

// streamPaths are the exact GET endpoints the browser UI consumes with
// EventSource or a WebSocket handshake (see web/src/api/endpoints.ts).
var streamPaths = []string{
	"/api/v1/query/tail",
	"/api/v1/terminal/ws",
}

// streamPathSuffixes cover the parameterised streaming endpoints.
var streamPathSuffixes = []string{
	"/ssl/certbot/stream",
	"/run/stream",
}

func isStreamRequest(c *gin.Context) bool {
	// Only GET requests may bypass the gate: the streaming routes above are
	// GET-only, so a mutating call to one of those paths must still
	// authenticate.
	if c.Request.Method != http.MethodGet {
		return false
	}
	path := c.Request.URL.Path
	for _, streamPath := range streamPaths {
		if path == streamPath {
			return true
		}
	}
	for _, suffix := range streamPathSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}
