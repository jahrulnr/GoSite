package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler serves liveness checks.
type HealthHandler struct{}

// NewHealthHandler returns a health HTTP handler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health reports service readiness without authentication.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
