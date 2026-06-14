package frontend

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Register mounts embedded SPA assets and history-mode fallback on engine.
// API routes must be registered before calling Register.
func Register(engine *gin.Engine, dist fs.FS) {
	if dist == nil {
		return
	}

	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		sub = dist
	}

	assets, _ := fs.Sub(sub, "assets")
	if assets != nil {
		engine.StaticFS("/assets", http.FS(assets))
	}

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api") {
			c.Status(http.StatusNotFound)
			return
		}

		if path != "/" && !strings.HasPrefix(path, "/.") {
			if data, err := fs.ReadFile(sub, strings.TrimPrefix(path, "/")); err == nil {
				ctype := contentType(path)
				c.Data(http.StatusOK, ctype, data)
				return
			}
		}

		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
}

func contentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(path, ".woff2"):
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}
