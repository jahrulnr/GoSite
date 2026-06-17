package nginxlite_test

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/observability/nginxlite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleBody = `Active connections: 291
server accepts handled requests
 92 66 45
Reading: 6 Writing: 179 Waiting: 106
`

func TestParseStubStatus(t *testing.T) {
	t.Parallel()
	got, err := nginxlite.ParseStubStatus(sampleBody)
	require.NoError(t, err)
	assert.Equal(t, 291, got.Active)
	assert.Equal(t, int64(92), got.Accepts)
	assert.Equal(t, int64(66), got.Handled)
	assert.Equal(t, int64(45), got.Requests)
	assert.Equal(t, 6, got.Reading)
	assert.Equal(t, 179, got.Writing)
	assert.Equal(t, 106, got.Waiting)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	return db
}

func TestNginxLite_CollectorInsertsSample(t *testing.T) {
	t.Parallel()
	repo := sqlite.NewNginxStatusRepository(openTestDB(t))
	collector := nginxlite.NewCollector("http://127.0.0.1/nginx_status", repo, 14)
	collector.SetHTTPClient(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(sampleBody)),
			Header:     make(http.Header),
		}, nil
	}))
	fixed := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	collector.SetNowFunc(func() time.Time { return fixed })

	require.NoError(t, collector.Collect(context.Background()))
	latest, ok, err := repo.LatestSample(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 291, latest.Active)
	assert.Equal(t, fixed, latest.SampleTS.UTC())
}

func TestNginxLite_SeriesRequestRate(t *testing.T) {
	t.Parallel()
	repo := sqlite.NewNginxStatusRepository(openTestDB(t))
	ctx := context.Background()
	base := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	require.NoError(t, repo.InsertSample(ctx, sqlite.NginxStatusSample{
		SampleTS: base, Active: 10, Requests: 100,
	}))
	require.NoError(t, repo.InsertSample(ctx, sqlite.NginxStatusSample{
		SampleTS: base.Add(30 * time.Second), Active: 20, Requests: 160,
	}))

	svc := nginxlite.NewService(repo, nil, "")
	svc.SetNowFunc(func() time.Time { return base.Add(time.Hour) })
	series, err := svc.Series(ctx, "1h")
	require.NoError(t, err)
	require.Len(t, series.RequestRate, 2)
	assert.Equal(t, 0.0, series.RequestRate[0][1])
	assert.Equal(t, 2.0, series.RequestRate[1][1])
}

func TestNginxLite_SeriesCounterReset(t *testing.T) {
	t.Parallel()
	repo := sqlite.NewNginxStatusRepository(openTestDB(t))
	ctx := context.Background()
	base := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	require.NoError(t, repo.InsertSample(ctx, sqlite.NginxStatusSample{
		SampleTS: base, Active: 10, Requests: 1_000_000,
	}))
	require.NoError(t, repo.InsertSample(ctx, sqlite.NginxStatusSample{
		SampleTS: base.Add(30 * time.Second), Active: 5, Requests: 100,
	}))

	svc := nginxlite.NewService(repo, nil, "")
	svc.SetNowFunc(func() time.Time { return base.Add(time.Hour) })
	series, err := svc.Series(ctx, "1h")
	require.NoError(t, err)
	require.Len(t, series.RequestRate, 2)
	assert.Nil(t, series.RequestRate[1][1])
}

func TestNginxLite_CurrentCounterReset(t *testing.T) {
	t.Parallel()
	repo := sqlite.NewNginxStatusRepository(openTestDB(t))
	ctx := context.Background()
	base := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	require.NoError(t, repo.InsertSample(ctx, sqlite.NginxStatusSample{
		SampleTS: base, Accepts: 500, Handled: 500, Requests: 1_000_000,
	}))
	require.NoError(t, repo.InsertSample(ctx, sqlite.NginxStatusSample{
		SampleTS: base.Add(30 * time.Second), Accepts: 10, Handled: 10, Requests: 50,
	}))

	svc := nginxlite.NewService(repo, nil, "")
	current, err := svc.Current(ctx)
	require.NoError(t, err)
	assert.True(t, current.CounterReset)
	assert.Nil(t, current.RequestRatePerSec)
	assert.Equal(t, int64(0), current.DroppedConnections)
}
