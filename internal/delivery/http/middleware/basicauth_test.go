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
