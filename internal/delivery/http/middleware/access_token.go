package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/auth"
	pluginsvc "github.com/jahrulnr/gosite/internal/service/plugin"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/jahrulnr/gosite/pkg/pluginperm"
)

type authKind string

const (
	AuthKindSession     authKind = "session"
	AuthKindAccessToken authKind = "access_token"

	authKindKey       = "gosite_auth_kind"
	accessTokenKey    = "gosite_access_token"
	accessScopesKey   = "gosite_access_scopes"
	accessTokenHeader = "X-Gosite-Access-Token"
)

// RequireSessionOrAccessToken accepts panel session or integration access token.
func RequireSessionOrAccessToken(authSvc *auth.Service, tokens *pluginsvc.IntegrationTokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		plaintext := strings.TrimSpace(c.GetHeader(accessTokenHeader))
		if plaintext != "" {
			if auth.SessionFromRequest(c.Request) != "" {
				abortUnauthorized(c, apperror.New(apperror.CodeUnauthorized, "use session or access token, not both"))
				return
			}
			authToken, err := tokens.Authenticate(c.Request.Context(), plaintext)
			if err != nil {
				abortUnauthorized(c, apperror.From(err))
				return
			}
			ctx := context.WithValue(c.Request.Context(), authKindKey, AuthKindAccessToken)
			ctx = context.WithValue(ctx, accessTokenKey, authToken.Row)
			ctx = context.WithValue(ctx, accessScopesKey, authToken.Scopes)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			if c.Writer.Status() < 400 {
				tokens.RecordUse(c.Request.Context(), authToken.Row.ID, c.Request.URL.Path, clientIP(c))
			}
			return
		}

		token := auth.SessionFromRequest(c.Request)
		if token == "" {
			abortUnauthorized(c, apperror.New(apperror.CodeUnauthorized, "authentication required"))
			return
		}
		if _, ok := authSvc.SessionUserID(token); !ok {
			abortUnauthorized(c, apperror.New(apperror.CodeSessionExpired, "session expired or invalid"))
			return
		}
		ctx := context.WithValue(c.Request.Context(), authKindKey, AuthKindSession)
		ctx = context.WithValue(ctx, sessionTokenKey, token)
		c.Request = c.Request.WithContext(ctx)
		c.Set(sessionTokenKey, token)
		c.Next()
	}
}

// RequireSessionOnly rejects integration access tokens (token admin CRUD).
func RequireSessionOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if AuthKind(c.Request.Context()) == AuthKindAccessToken {
			abortForbidden(c, apperror.New(apperror.CodeForbidden, "session required"))
			return
		}
		c.Next()
	}
}

// RequireAccessTokenOnly validates integration token without session fallback.
func RequireAccessTokenOnly(tokens *pluginsvc.IntegrationTokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		plaintext := strings.TrimSpace(c.GetHeader(accessTokenHeader))
		if plaintext == "" {
			abortUnauthorized(c, apperror.New(apperror.CodeUnauthorized, "access token required"))
			return
		}
		authToken, err := tokens.Authenticate(c.Request.Context(), plaintext)
		if err != nil {
			abortUnauthorized(c, apperror.From(err))
			return
		}
		ctx := context.WithValue(c.Request.Context(), authKindKey, AuthKindAccessToken)
		ctx = context.WithValue(ctx, accessTokenKey, authToken.Row)
		ctx = context.WithValue(ctx, accessScopesKey, authToken.Scopes)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		if c.Writer.Status() < 400 {
			tokens.RecordUse(c.Request.Context(), authToken.Row.ID, c.Request.URL.Path, clientIP(c))
		}
	}
}

// RequireScope enforces scope for access-token auth; sessions bypass.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if AuthKind(c.Request.Context()) != AuthKindAccessToken {
			c.Next()
			return
		}
		scopes, _ := c.Request.Context().Value(accessScopesKey).([]string)
		if !pluginperm.HasScope(scopes, scope) {
			abortForbidden(c, apperror.New(apperror.CodeForbidden, "missing scope: "+scope))
			return
		}
		c.Next()
	}
}

// AuthKind returns how the request was authenticated.
func AuthKind(ctx context.Context) authKind {
	kind, _ := ctx.Value(authKindKey).(authKind)
	return kind
}

// AccessTokenFromContext returns the authenticated token row when present.
func AccessTokenFromContext(ctx context.Context) (sqlite.PluginAccessToken, bool) {
	row, ok := ctx.Value(accessTokenKey).(sqlite.PluginAccessToken)
	return row, ok
}

// AccessScopesFromContext returns scopes for the authenticated access token.
func AccessScopesFromContext(ctx context.Context) []string {
	scopes, _ := ctx.Value(accessScopesKey).([]string)
	return scopes
}

// SessionTokenFromContext returns session token from request context.
func SessionTokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(sessionTokenKey).(string)
	return token
}

func abortForbidden(c *gin.Context, err *apperror.Error) {
	c.AbortWithStatusJSON(http.StatusForbidden, err.Body())
}

func clientIP(c *gin.Context) string {
	if forwarded := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(c.ClientIP())
}
