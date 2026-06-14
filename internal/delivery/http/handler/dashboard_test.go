package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jahrulnr/gosite/internal/delivery/http/handler"
	"github.com/jahrulnr/gosite/internal/observability/grafanalite"
	"github.com/jahrulnr/gosite/internal/observability/splunklite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/ssl"
	"github.com/jahrulnr/gosite/internal/service/system"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "migrations"))
}

func TestDashboard_ContainsAllSections(t *testing.T) {
	t.Parallel()

	procFS := system.MapFS{Files: map[string][]byte{
		"/proc/loadavg":   []byte("0.50 0.25 0.10 1/50 1\n"),
		"/proc/cpuinfo":   []byte("processor\t: 0\n"),
		"/proc/meminfo":   []byte("MemTotal: 1024 kB\nMemAvailable: 512 kB\n"),
		"/proc/net/dev":   []byte("Inter-| Receive | Transmit\n eth0: 100 1 0 0 0 0 0 0 200 2 0 0 0 0 0 0 0\n"),
		"/proc/diskstats": []byte("8 0 sda 1 0 10 0 0 20 0 0 0 0 0 0 0\n"),
	}}
	logDir := t.TempDir()
	systemSvc := system.NewService(logDir, procFS, nil)

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	auditRepo := sqlite.NewAuditRepository(db)
	jobRepo := sqlite.NewJobRepository(db)
	logRepo := sqlite.NewLogEventRepository(db)
	savedRepo := sqlite.NewSavedQueryRepository(db)
	splunkSvc := splunklite.NewService(auditRepo, jobRepo, logRepo, savedRepo, 90, 14)
	metricsRepo := sqlite.NewTrafficMetricsRepository(db)
	grafanaSvc := grafanalite.NewService(metricsRepo)

	websiteRepo := sqlite.NewWebsiteRepository(db)
	sslSvc := ssl.NewService(websiteRepo, jobRepo, nil)

	dash := handler.NewDashboardHandler(systemSvc, sslSvc, splunkSvc, grafanaSvc)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	dash.Get(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	for _, key := range []string{"system", "traffic_summary", "ssl_expiring", "recent_audit"} {
		_, ok := body[key]
		assert.True(t, ok, "missing dashboard key %s", key)
	}
}

func TestDashboard_SystemSectionHasCPU(t *testing.T) {
	t.Parallel()

	procFS := system.MapFS{Files: map[string][]byte{
		"/proc/loadavg": []byte("1.00 0.50 0.25 2/100 999\n"),
		"/proc/cpuinfo": []byte("processor\t: 0\nprocessor\t: 1\n"),
		"/proc/meminfo": []byte("MemTotal: 8192 kB\nMemAvailable: 4096 kB\n"),
	}}
	systemSvc := system.NewService(t.TempDir(), procFS, nil)

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	auditRepo := sqlite.NewAuditRepository(db)
	splunkSvc := splunklite.NewService(auditRepo, sqlite.NewJobRepository(db), sqlite.NewLogEventRepository(db), sqlite.NewSavedQueryRepository(db), 90, 14)
	grafanaSvc := grafanalite.NewService(sqlite.NewTrafficMetricsRepository(db))
	sslSvc := ssl.NewService(sqlite.NewWebsiteRepository(db), sqlite.NewJobRepository(db), nil)

	dash := handler.NewDashboardHandler(systemSvc, sslSvc, splunkSvc, grafanaSvc)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	dash.Get(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		System struct {
			CPU float64 `json:"cpu"`
		} `json:"system"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 50.0, body.System.CPU)
}

func TestDashboard_RecentAuditFromSplunk(t *testing.T) {
	t.Parallel()

	procFS := system.MapFS{Files: map[string][]byte{
		"/proc/loadavg": []byte("0 0 0 0/0 0\n"),
		"/proc/cpuinfo": []byte("processor\t: 0\n"),
		"/proc/meminfo": []byte("MemTotal: 1024 kB\nMemAvailable: 512 kB\n"),
	}}
	systemSvc := system.NewService(t.TempDir(), procFS, nil)

	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))

	auditRepo := sqlite.NewAuditRepository(db)
	require.NoError(t, auditRepo.Write(context.Background(), sqlite.AuditLog{
		UserEmail: "admin@demo.com",
		Action:    "login",
		Status:    "ok",
		Message:   "signed in",
	}))

	splunkSvc := splunklite.NewService(auditRepo, sqlite.NewJobRepository(db), sqlite.NewLogEventRepository(db), sqlite.NewSavedQueryRepository(db), 90, 14)
	dash := handler.NewDashboardHandler(systemSvc, ssl.NewService(sqlite.NewWebsiteRepository(db), sqlite.NewJobRepository(db), nil), splunkSvc, grafanalite.NewService(sqlite.NewTrafficMetricsRepository(db)))

	rec := httptest.NewRecorder()
	dash.Get(rec, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		RecentAudit []struct {
			Action string `json:"action"`
		} `json:"recent_audit"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.RecentAudit, 1)
	assert.Equal(t, "login", body.RecentAudit[0].Action)
}
