package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/delivery/http/handler"
	"github.com/jahrulnr/gosite/internal/observability/grafanalite"
	"github.com/jahrulnr/gosite/internal/observability/nginxlite"
	"github.com/jahrulnr/gosite/internal/observability/splunklite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupObservabilityHandler(t *testing.T) *handler.ObservabilityHandler {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	auditRepo := sqlite.NewAuditRepository(db)
	jobRepo := sqlite.NewJobRepository(db)
	logRepo := sqlite.NewLogEventRepository(db)
	savedRepo := sqlite.NewSavedQueryRepository(db)
	require.NoError(t, auditRepo.Write(httptest.NewRequest(http.MethodGet, "/", nil).Context(), sqlite.AuditLog{
		UserEmail:    "admin@demo.com",
		Action:       "website.create",
		ResourceType: "website",
		ResourceID:   "1",
		Domain:       "example.test",
		Status:       "ok",
		Message:      "created",
	}))

	splunkSvc := splunklite.NewService(auditRepo, jobRepo, logRepo, savedRepo, 90, 14)
	metricsRepo := sqlite.NewTrafficMetricsRepository(db)
	return handler.NewObservabilityHandler(splunkSvc, nil, nil, grafanalite.NewService(metricsRepo), nil)
}

func TestObservability_QueryGetBatch(t *testing.T) {
	t.Parallel()
	h := setupObservabilityHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/query?source=audit&q=action%3Awebsite.*&limit=10", nil)
	rec := httptest.NewRecorder()
	h.QueryGet(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body splunklite.QueryResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 1, body.Hits)
	require.Len(t, body.Events, 1)
	assert.Equal(t, "website.create", body.Events[0].Action)
}

func TestObservability_QueryGetSSEStream(t *testing.T) {
	t.Parallel()
	h := setupObservabilityHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/query?source=audit&q=created&stream=sse", nil)
	rec := httptest.NewRecorder()
	h.QueryGet(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	body := rec.Body.String()
	assert.NotContains(t, body, `"type":"ingesting"`)
	assert.Contains(t, body, `"type":"meta"`)
	assert.Contains(t, body, `"type":"event"`)
	assert.Contains(t, body, `"type":"done"`)
	assert.True(t, strings.Contains(body, "data: "))
}

func TestObservability_NginxCurrent(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	repo := sqlite.NewNginxStatusRepository(db)
	require.NoError(t, repo.InsertSample(httptest.NewRequest(http.MethodGet, "/", nil).Context(), sqlite.NginxStatusSample{
		SampleTS: time.Now().UTC(), Active: 12, Requests: 50,
	}))
	h := handler.NewObservabilityHandler(nil, nil, nil, nil, nginxlite.NewService(repo, nil, ""))

	req := httptest.NewRequest(http.MethodGet, "/metrics/nginx/current", nil)
	rec := httptest.NewRecorder()
	h.NginxCurrent(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body nginxlite.CurrentResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.True(t, body.Available)
	assert.Equal(t, 12, body.Active)
}
