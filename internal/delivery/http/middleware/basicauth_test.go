package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/delivery/http/middleware"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestBasicAuth_DisabledBypass(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		AuthEnable: false,
		AuthUser:   "admin",
		AuthPass:   "secret",
	}

	router := gin.New()
	router.Use(middleware.BasicAuth(cfg))
	router.GET("/api/v1/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBasicAuth_RequiredWhenEnabled(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		AuthEnable: true,
		AuthUser:   "panel",
		AuthPass:   "secret",
	}

	router := gin.New()
	router.Use(middleware.BasicAuth(cfg))
	router.GET("/api/v1/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `Basic realm="Access denied"`)

	var body apperror.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, apperror.CodeBasicAuthRequired, body.Error.Code)
}

func TestBasicAuth_ValidCredentials(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		AuthEnable: true,
		AuthUser:   "panel",
		AuthPass:   "secret",
	}

	router := gin.New()
	router.Use(middleware.BasicAuth(cfg))
	router.GET("/api/v1/protected", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/protected", nil)
	req.SetBasicAuth("panel", "secret")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBasicAuth_StreamBypassLimitedToStreamPaths(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		AuthEnable: true,
		AuthUser:   "panel",
		AuthPass:   "secret",
	}

	router := gin.New()
	router.Use(middleware.BasicAuth(cfg))
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	for _, path := range []string{
		"/api/v1/query/tail",
		"/api/v1/terminal/ws",
		"/api/v1/websites/7/ssl/certbot/stream",
		"/api/v1/cronjobs/3/run/stream",
		"/api/v1/query",
		"/api/v1/logs",
		"/api/v1/auth/login",
	} {
		router.GET(path, ok)
	}
	router.POST("/api/v1/query/tail", ok)
	router.POST("/api/v1/auth/login", ok)

	t.Run("stream paths bypass without credentials", func(t *testing.T) {
		t.Parallel()
		for _, path := range []string{
			"/api/v1/query/tail",
			"/api/v1/terminal/ws",
			"/api/v1/websites/7/ssl/certbot/stream",
			"/api/v1/cronjobs/3/run/stream",
		} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "path %s should bypass BasicAuth", path)
		}
	})

	t.Run("stream headers and query do not bypass other paths", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name   string
			method string
			path   string
			accept string
			query  string
			extra  map[string]string
		}{
			{name: "login with accept sse", method: http.MethodPost, path: "/api/v1/auth/login", accept: "text/event-stream"},
			{name: "login with stream=sse", method: http.MethodPost, path: "/api/v1/auth/login", query: "stream=sse"},
			{name: "login metadata with accept ndjson", method: http.MethodGet, path: "/api/v1/auth/login", accept: "application/x-ndjson"},
			{name: "query with accept sse", method: http.MethodGet, path: "/api/v1/query", accept: "text/event-stream"},
			{name: "query with stream=ndjson", method: http.MethodGet, path: "/api/v1/query", query: "stream=ndjson"},
			{name: "logs with accept sse", method: http.MethodGet, path: "/api/v1/logs", accept: "text/event-stream"},
			{name: "websocket upgrade on other path", method: http.MethodGet, path: "/api/v1/query", extra: map[string]string{"Upgrade": "websocket", "Connection": "Upgrade"}},
			{name: "post to stream path", method: http.MethodPost, path: "/api/v1/query/tail"},
		}
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				url := tc.path
				if tc.query != "" {
					url += "?" + tc.query
				}
				req := httptest.NewRequest(tc.method, url, nil)
				if tc.accept != "" {
					req.Header.Set("Accept", tc.accept)
				}
				for k, v := range tc.extra {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusUnauthorized, rec.Code, "non-stream request must stay behind BasicAuth")
			})
		}
	})
}
