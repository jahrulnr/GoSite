package handler

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jahrulnr/gosite/internal/terminal"
	"github.com/stretchr/testify/assert"
)

func TestTerminalHandlerSnapshotEncoding(t *testing.T) {
	dir := t.TempDir()
	hub := terminal.NewHub(terminal.HubConfig{
		StickyTTL:    5 * time.Second,
		DumpDir:      dir,
		DefaultShell: "/bin/sh",
		DefaultCwd:   dir,
	})

	id := "snap-encoding"
	dumpPath := filepath.Join(dir, "gosite-term-"+id+".log")
	if err := os.WriteFile(dumpPath, []byte("seed content for snapshot test\n"), 0o644); err != nil {
		t.Fatalf("seed dump: %v", err)
	}
	if _, _, err := hub.AttachOrCreate(context.Background(), 1, id, terminal.CreateOptions{}); err != nil {
		t.Fatalf("AttachOrCreate: %v", err)
	}
	defer hub.Kill(id)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/snapshot/:id", func(c *gin.Context) {
		s, ok := hub.Get(c.Param("id"))
		if !ok {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		data, first, end := s.Snapshot()
		c.JSON(200, gin.H{
			"session_id": c.Param("id"),
			"bytes":      len(data),
			"first_seq":  first,
			"end_seq":    end,
			"data_b64":   base64.StdEncoding.EncodeToString(data),
		})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/snapshot/"+id, nil)
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data_b64") {
		t.Errorf("missing data_b64 in response: %s", body)
	}
	// Extract data_b64 between quotes.
	idx := strings.Index(body, `"data_b64":"`)
	if idx < 0 {
		t.Fatalf("data_b64 not found in body: %s", body)
	}
	start := idx + len(`"data_b64":"`)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("malformed data_b64: %s", body)
	}
	decoded, err := base64.StdEncoding.DecodeString(body[start : start+end])
	if err != nil {
		t.Fatalf("decode b64: %v", err)
	}
	if !strings.Contains(string(decoded), "seed content") {
		t.Errorf("snapshot missing seeded content: %q", string(decoded))
	}
}

func TestTerminalHandlerKillMissingReturnsNotFound(t *testing.T) {
	hub := terminal.NewHub(terminal.HubConfig{
		StickyTTL:    5 * time.Second,
		DumpDir:      t.TempDir(),
		DefaultShell: "/bin/sh",
		DefaultCwd:   t.TempDir(),
	})
	if err := hub.Kill("does-not-exist"); err == nil {
		t.Fatal("expected error killing missing session")
	}
}

func TestTerminalHandlerCheckOrigin(t *testing.T) {
	t.Parallel()

	h := NewTerminalHandler(nil, nil, nil, []string{"https://panel.example.com", "admin.example.com"})

	cases := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{name: "no origin (non-browser client)", host: "panel.local:8080", want: true},
		{name: "same origin", origin: "https://panel.local:8080", host: "panel.local:8080", want: true},
		{name: "same origin different scheme", origin: "http://panel.local:8080", host: "panel.local:8080", want: true},
		{name: "sibling subdomain is not same-origin", origin: "https://evil.panel.local:8080", host: "panel.local:8080", want: false},
		{name: "foreign origin", origin: "https://attacker.example.com", host: "panel.local:8080", want: false},
		{name: "configured origin", origin: "https://panel.example.com", host: "panel.local:8080", want: true},
		{name: "configured bare host", origin: "https://admin.example.com", host: "panel.local:8080", want: true},
		{name: "malformed origin", origin: "://bad", host: "panel.local:8080", want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/terminal/ws", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			assert.Equal(t, tc.want, h.checkOrigin(req))
		})
	}
}
