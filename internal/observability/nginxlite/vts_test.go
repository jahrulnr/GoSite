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

const vtsSampleJSON = `{
  "serverZones": {
    "example.test": {
      "requestCounter": 120,
      "inBytes": 4000,
      "outBytes": 9000,
      "requestMsec": 32.5
    }
  },
  "upstreamZones": {
    "backend": [
      {
        "server": "127.0.0.1:3000",
        "requestCounter": 80,
        "inBytes": 2000,
        "outBytes": 5000,
        "responseMsec": 18.2,
        "down": false
      }
    ]
  }
}`

func TestParseVTSJSON(t *testing.T) {
	t.Parallel()
	got, err := nginxlite.ParseVTSJSON([]byte(vtsSampleJSON))
	require.NoError(t, err)
	require.Len(t, got.Servers, 1)
	assert.Equal(t, "example.test", got.Servers[0].ServerName)
	assert.Equal(t, 120, got.Servers[0].Requests)
	require.Len(t, got.Upstreams, 1)
	assert.Equal(t, "backend", got.Upstreams[0].UpstreamName)
	assert.Equal(t, "127.0.0.1:3000", got.Upstreams[0].ServerAddr)
}

func openVTSTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	return db
}

func TestNginxLite_VTSCollectorInsertsSamples(t *testing.T) {
	t.Parallel()
	repo := sqlite.NewNginxVTSRepository(openVTSTestDB(t))
	collector := nginxlite.NewVTSCollector("http://127.0.0.1/status/format/json", repo, 14)
	collector.SetHTTPClient(roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(vtsSampleJSON)),
			Header:     make(http.Header),
		}, nil
	}))
	fixed := time.Date(2026, 6, 17, 13, 0, 0, 0, time.UTC)
	collector.SetNowFunc(func() time.Time { return fixed })

	require.NoError(t, collector.Collect(context.Background()))
	servers, err := repo.TopServersAtLatest(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Equal(t, "example.test", servers[0].ServerName)
	upstreams, err := repo.TopUpstreamsAtLatest(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, upstreams, 1)
	assert.Equal(t, "backend", upstreams[0].UpstreamName)
}

func TestNginxLite_VTSServiceTopRows(t *testing.T) {
	t.Parallel()
	repo := sqlite.NewNginxVTSRepository(openVTSTestDB(t))
	ctx := context.Background()
	ts := time.Date(2026, 6, 17, 13, 0, 0, 0, time.UTC)
	require.NoError(t, repo.InsertServerSample(ctx, sqlite.VTSServerSample{
		SampleTS: ts, ServerName: "a.test", Requests: 10,
	}))
	require.NoError(t, repo.InsertUpstreamSample(ctx, sqlite.VTSUpstreamSample{
		SampleTS: ts, UpstreamName: "backend", ServerAddr: "127.0.0.1:1", Requests: 5,
	}))
	svc := nginxlite.NewService(nil, repo, "http://127.0.0.1/status/format/json")
	servers, err := svc.VTSServers(ctx, 5)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	upstreams, err := svc.VTSUpstreams(ctx, 5)
	require.NoError(t, err)
	require.Len(t, upstreams, 1)
}
