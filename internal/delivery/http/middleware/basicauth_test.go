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

func TestBasicAuth_StreamBypass(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		AuthEnable: true,
		AuthUser:   "panel",
		AuthPass:   "secret",
	}

	router := gin.New()
	router.Use(middleware.BasicAuth(cfg))
	for _, path := range []string{"/api/v1/query/tail", "/api/v1/query", "/api/v1/logs"} {
		router.GET(path, func(c *gin.Context) { c.Status(http.StatusOK) })
	}

	cases := []struct {
		name   string
		path   string
		accept string
		query  string
	}{
		{"tail path", "/api/v1/query/tail", "", ""},
		{"stream=sse", "/api/v1/query", "", "stream=sse"},
		{"stream=ndjson", "/api/v1/query", "", "stream=ndjson"},
		{"accept sse", "/api/v1/query", "text/event-stream", ""},
		{"accept ndjson", "/api/v1/query", "application/x-ndjson", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			url := tc.path
			if tc.query != "" {
				url += "?" + tc.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusOK, rec.Code, "expected stream request to bypass BasicAuth")
		})
	}
}
