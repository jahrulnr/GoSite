package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/service/auth"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

const sessionTokenKey = "session_token"

// RequireSession ensures the request has a valid panel session cookie.
func RequireSession(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := auth.SessionFromRequest(c.Request)
		if token == "" {
			abortUnauthorized(c, apperror.New(apperror.CodeUnauthorized, "authentication required"))
			return
		}

		if _, ok := svc.SessionUserID(token); !ok {
			abortUnauthorized(c, apperror.New(apperror.CodeSessionExpired, "session expired or invalid"))
			return
		}

		c.Set(sessionTokenKey, token)
		c.Next()
	}
}

// SessionToken returns the authenticated session token stored by RequireSession.
func SessionToken(c *gin.Context) string {
	token, _ := c.Get(sessionTokenKey)
	value, _ := token.(string)
	return value
}

func abortUnauthorized(c *gin.Context, err *apperror.Error) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, err.Body())
}
