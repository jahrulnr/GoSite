package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

const basicAuthRealm = "Access denied"

// BasicAuth gates requests when AUTH_ENABLE is true.
func BasicAuth(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cfg.AuthEnable {
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
