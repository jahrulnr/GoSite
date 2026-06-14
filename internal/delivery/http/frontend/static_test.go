package frontend_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/delivery/http/frontend"
	"github.com/stretchr/testify/assert"
)

func TestRegister_ServesIndexAndSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	engine.GET("/api/v1/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	dist := fstest.MapFS{
		"dist/index.html": &fstest.MapFile{
			Data: []byte("<!DOCTYPE html><html><body>GoSite</body></html>"),
		},
	}
	frontend.Register(engine, dist)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "html")

	req2 := httptest.NewRequest(http.MethodGet, "/websites", nil)
	rec2 := httptest.NewRecorder()
	engine.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "html")

	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	rec3 := httptest.NewRecorder()
	engine.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusNotFound, rec3.Code)
}
