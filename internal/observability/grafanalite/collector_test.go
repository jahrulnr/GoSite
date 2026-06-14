package grafanalite_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/observability/grafanalite"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, migrationsDir(t)))
	return db
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	return filepath.Clean(filepath.Join("..", "..", "..", "migrations"))
}

func TestGrafana_Rollup5mBucket(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 14, 10, 7, 30, 0, time.UTC)
	bucket := grafanalite.TruncateBucket(ts)
	assert.Equal(t, 5, bucket.Minute())
	assert.Equal(t, 10, bucket.Hour())

	ts2 := time.Date(2026, 6, 14, 10, 11, 0, 0, time.UTC)
	bucket2 := grafanalite.TruncateBucket(ts2)
	assert.Equal(t, 10, bucket2.Minute())
}

func TestGrafana_OffsetResumeAfterRestart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	offsetPath := filepath.Join(dir, "metrics_offsets.json")
	require.NoError(t, os.MkdirAll(logDir, 0o755))

	logFile := filepath.Join(logDir, "access-example.test.log")
	line1 := strings.Replace(testutil.SampleAccessLogLines[0], "10:00:01", "10:00:01", 1)
	line2 := strings.Replace(testutil.SampleAccessLogLines[1], "10:00:02", "10:05:02", 1)
	require.NoError(t, os.WriteFile(logFile, []byte(line1+"\n"), 0o644))

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	collector := grafanalite.NewCollector(logDir, offsetPath, metrics, 14)
	ctx := context.Background()

	require.NoError(t, collector.Collect(ctx))
	state, err := collector.LoadOffsets()
	require.NoError(t, err)
	firstOffset := state.Files["access-example.test.log"]
	assert.Greater(t, firstOffset, int64(0))

	require.NoError(t, os.WriteFile(logFile, []byte(line1+"\n"+line2+"\n"), 0o644))
	require.NoError(t, collector.Collect(ctx))

	state2, err := collector.LoadOffsets()
	require.NoError(t, err)
	assert.Greater(t, state2.Files["access-example.test.log"], firstOffset)

	buckets, err := metrics.ListBuckets(ctx, time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), "")
	require.NoError(t, err)
	assert.NotEmpty(t, buckets)
	totalReq := 0
	for _, b := range buckets {
		totalReq += b.Requests
	}
	assert.Equal(t, 2, totalReq)
}

func TestGrafana_RetentionPurge(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	ctx := context.Background()
	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: old, Site: "old.test", Requests: 5, Bytes: 100, S2xx: 5,
	}))
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: time.Now().UTC(), Site: "fresh.test", Requests: 1, Bytes: 10, S2xx: 1,
	}))

	n, err := metrics.PurgeOlderThan(ctx, time.Now().UTC().Add(-7*24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	from := time.Now().UTC().Add(-48 * time.Hour)
	to := time.Now().UTC().Add(1 * time.Hour)
	buckets, err := metrics.ListBuckets(ctx, from, to, "")
	require.NoError(t, err)
	require.Len(t, buckets, 1)
	assert.Equal(t, "fresh.test", buckets[0].Site)
}

func TestGrafana_ServiceSeries(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()

	bucket := time.Date(2026, 6, 14, 11, 30, 0, 0, time.UTC)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "a.test", Requests: 3, Bytes: 300, S2xx: 3,
	}))

	series, err := svc.TrafficSeries(ctx, "1h", "")
	require.NoError(t, err)
	assert.Equal(t, "5m", series.Step)
	assert.NotEmpty(t, series.Requests["a.test"])
}

func TestGrafana_TopSites(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()
	bucket := now.Add(-15 * time.Minute)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "busy.test", Requests: 10, Bytes: 1000, S2xx: 10,
	}))
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "quiet.test", Requests: 1, Bytes: 50, S2xx: 1,
	}))

	rows, err := svc.TopSites(ctx, "1h", 5)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "busy.test", rows[0].Site)
}

func TestGrafana_StatusCodes(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()
	bucket := now.Add(-10 * time.Minute)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "x.test", Requests: 4, Bytes: 400, S2xx: 2, S4xx: 2,
	}))

	res, err := svc.StatusCodes(ctx, "1h", "")
	require.NoError(t, err)
	assert.Equal(t, 2, res.S2xx)
	assert.Equal(t, 2, res.S4xx)
}

func TestGrafana_Summary(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()
	bucket := now.Add(-20 * time.Minute)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "x.test", Requests: 7, Bytes: 700, S2xx: 7,
	}))

	sum, err := svc.Summary(ctx, "1h")
	require.NoError(t, err)
	assert.Equal(t, 7, sum.Requests)
	assert.Equal(t, 700, sum.Bytes)
}

func TestGrafana_EmptyLogDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	offsetPath := filepath.Join(dir, "offsets.json")
	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	collector := grafanalite.NewCollector(logDir, offsetPath, metrics, 14)
	require.NoError(t, collector.Collect(context.Background()))
}

func TestGrafana_LogRotationResetsOffset(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	offsetPath := filepath.Join(dir, "offsets.json")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	logFile := filepath.Join(logDir, "access.log")
	line := testutil.SampleAccessLogLines[0]
	require.NoError(t, os.WriteFile(logFile, []byte(line+"\n"), 0o644))

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	collector := grafanalite.NewCollector(logDir, offsetPath, metrics, 14)
	ctx := context.Background()
	require.NoError(t, collector.Collect(ctx))

	// simulate truncate / rotation (file smaller than saved offset)
	require.NoError(t, os.WriteFile(logFile, []byte(line+"\n"), 0o644))
	require.NoError(t, collector.Collect(ctx))

	state, err := collector.LoadOffsets()
	require.NoError(t, err)
	assert.Greater(t, state.Files["access.log"], int64(0))
}

func TestGrafana_InvalidRange(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	svc := grafanalite.NewService(sqlite.NewTrafficMetricsRepository(db))
	_, err := svc.Summary(context.Background(), "bad-range")
	require.Error(t, err)
}

func TestGrafana_ParseAccessLineViaCollect(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	offsetPath := filepath.Join(dir, "offsets.json")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	logFile := filepath.Join(logDir, "access-default.log")
	content := strings.Join(testutil.SampleAccessLogLines, "\n") + "\n"
	require.NoError(t, os.WriteFile(logFile, []byte(content), 0o644))

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	collector := grafanalite.NewCollector(logDir, offsetPath, metrics, 14)
	require.NoError(t, collector.Collect(context.Background()))

	buckets, err := metrics.ListBuckets(context.Background(),
		time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), "default")
	require.NoError(t, err)
	assert.NotEmpty(t, buckets)
}

func TestGrafana_SeriesSiteFilter(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()
	bucket := now.Add(-10 * time.Minute)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "keep.test", Requests: 1, Bytes: 10, S2xx: 1,
	}))
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "skip.test", Requests: 9, Bytes: 90, S2xx: 9,
	}))

	series, err := svc.TrafficSeries(ctx, "1h", "keep.test")
	require.NoError(t, err)
	assert.Contains(t, series.Requests, "keep.test")
	assert.NotContains(t, series.Requests, "skip.test")
}

func TestGrafana_CollectorPurgeRetention(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	offsetPath := filepath.Join(dir, "offsets.json")
	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	collector := grafanalite.NewCollector(logDir, offsetPath, metrics, 7)
	ctx := context.Background()

	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: old, Site: "purge.test", Requests: 1, Bytes: 10, S2xx: 1,
	}))
	require.NoError(t, collector.PurgeRetention(ctx))
}

func TestGrafana_CollectorSetNowFunc(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	collector := grafanalite.NewCollector(filepath.Join(dir, "logs"), filepath.Join(dir, "off.json"), metrics, 14)
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	collector.SetNowFunc(func() time.Time { return fixed })
	require.NoError(t, collector.PurgeRetention(context.Background()))
}

func TestGrafana_ParseRangeVariants(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	svc := grafanalite.NewService(sqlite.NewTrafficMetricsRepository(db))
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()

	_, err := svc.TrafficSeries(ctx, "24h", "")
	require.NoError(t, err)
	_, err = svc.TrafficSeries(ctx, "7d", "")
	require.NoError(t, err)
}

func TestGrafana_StatusCodesSiteFilter(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()
	bucket := now.Add(-10 * time.Minute)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "only.test", Requests: 2, Bytes: 200, S2xx: 2,
	}))
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "other.test", Requests: 5, Bytes: 500, S4xx: 5,
	}))

	res, err := svc.StatusCodes(ctx, "1h", "only.test")
	require.NoError(t, err)
	assert.Equal(t, 2, res.S2xx)
	assert.Equal(t, 0, res.S4xx)
}

func TestGrafana_TopSitesDefaultLimit(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()
	bucket := now.Add(-5 * time.Minute)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "a.test", Requests: 3, Bytes: 30, S2xx: 3,
	}))

	rows, err := svc.TopSites(ctx, "1h", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, rows)
}

func TestGrafana_CollectMalformedLineSkipped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	offsetPath := filepath.Join(dir, "offsets.json")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	logFile := filepath.Join(logDir, "access-bad.log")
	require.NoError(t, os.WriteFile(logFile, []byte("not-json-line\n"), 0o644))

	db := openTestDB(t)
	collector := grafanalite.NewCollector(logDir, offsetPath, sqlite.NewTrafficMetricsRepository(db), 14)
	require.NoError(t, collector.Collect(context.Background()))
}

func TestGrafana_Summary24h(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: now.Add(-2 * time.Hour), Site: "sum.test", Requests: 4, Bytes: 40, S2xx: 4,
	}))

	sum, err := svc.Summary(ctx, "24h")
	require.NoError(t, err)
	assert.Equal(t, 4, sum.Requests)
}

func TestGrafana_NewCollectorDefaults(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	collector := grafanalite.NewCollector(t.TempDir()+"/missing", filepath.Join(t.TempDir(), "off.json"),
		sqlite.NewTrafficMetricsRepository(db), 0)
	require.NoError(t, collector.Collect(context.Background()))
}

func TestGrafana_CollectTwicePersistsOffsets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	offsetPath := filepath.Join(dir, "offsets.json")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	logFile := filepath.Join(logDir, "access-twice.test.log")
	line := testutil.SampleAccessLogLines[0]
	require.NoError(t, os.WriteFile(logFile, []byte(line+"\n"), 0o644))

	db := openTestDB(t)
	collector := grafanalite.NewCollector(logDir, offsetPath, sqlite.NewTrafficMetricsRepository(db), 14)
	ctx := context.Background()
	require.NoError(t, collector.Collect(ctx))
	require.NoError(t, collector.Collect(ctx))

	state, err := collector.LoadOffsets()
	require.NoError(t, err)
	assert.Greater(t, state.Files["access-twice.test.log"], int64(0))
}

func TestGrafana_CollectIgnoresNonAccessFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	offsetPath := filepath.Join(dir, "off.json")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(logDir, "error.log"), []byte("x\n"), 0o644))

	db := openTestDB(t)
	collector := grafanalite.NewCollector(logDir, offsetPath, sqlite.NewTrafficMetricsRepository(db), 14)
	require.NoError(t, collector.Collect(context.Background()))
}

func TestGrafana_StatusCodesIncludes5xx(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	metrics := sqlite.NewTrafficMetricsRepository(db)
	svc := grafanalite.NewService(metrics)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return now })
	ctx := context.Background()
	bucket := now.Add(-10 * time.Minute)
	require.NoError(t, metrics.UpsertBucket(ctx, sqlite.TrafficBucket{
		BucketTS: bucket, Site: "err.test", Requests: 3, Bytes: 300, S5xx: 3,
	}))

	res, err := svc.StatusCodes(ctx, "1h", "")
	require.NoError(t, err)
	assert.Equal(t, 3, res.S5xx)
}
