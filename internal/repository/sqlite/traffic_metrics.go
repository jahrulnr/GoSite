package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// TrafficBucket is a pre-aggregated traffic metric row.
type TrafficBucket struct {
	ID       int64
	BucketTS time.Time
	Site     string
	Requests int
	Bytes    int
	S2xx     int
	S3xx     int
	S4xx     int
	S5xx     int
}

// TrafficMetricsRepository persists traffic metric buckets.
type TrafficMetricsRepository struct {
	db *sql.DB
}

// NewTrafficMetricsRepository returns a traffic metrics repository backed by db.
func NewTrafficMetricsRepository(db *sql.DB) *TrafficMetricsRepository {
	return &TrafficMetricsRepository{db: db}
}

// UpsertBucket inserts or increments a traffic bucket.
func (r *TrafficMetricsRepository) UpsertBucket(ctx context.Context, b TrafficBucket) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO traffic_metrics (bucket_ts, site, requests, bytes, s2xx, s3xx, s4xx, s5xx)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(bucket_ts, site) DO UPDATE SET
			requests = requests + excluded.requests,
			bytes = bytes + excluded.bytes,
			s2xx = s2xx + excluded.s2xx,
			s3xx = s3xx + excluded.s3xx,
			s4xx = s4xx + excluded.s4xx,
			s5xx = s5xx + excluded.s5xx
	`, b.BucketTS, b.Site, b.Requests, b.Bytes, b.S2xx, b.S3xx, b.S4xx, b.S5xx)
	if err != nil {
		return fmt.Errorf("upsert traffic bucket: %w", err)
	}
	return nil
}

// ListBuckets returns buckets in a time range optionally filtered by site.
func (r *TrafficMetricsRepository) ListBuckets(ctx context.Context, from, to time.Time, site string) ([]TrafficBucket, error) {
	query := `
		SELECT id, bucket_ts, site, requests, bytes, s2xx, s3xx, s4xx, s5xx
		FROM traffic_metrics
		WHERE bucket_ts >= ? AND bucket_ts < ?`
	args := []interface{}{from, to}
	if site != "" {
		query += ` AND site = ?`
		args = append(args, site)
	}
	query += ` ORDER BY bucket_ts ASC, site ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list traffic buckets: %w", err)
	}
	defer rows.Close()

	var out []TrafficBucket
	for rows.Next() {
		var b TrafficBucket
		var ts sql.NullTime
		if err := rows.Scan(&b.ID, &ts, &b.Site, &b.Requests, &b.Bytes, &b.S2xx, &b.S3xx, &b.S4xx, &b.S5xx); err != nil {
			return nil, fmt.Errorf("scan traffic bucket: %w", err)
		}
		if ts.Valid {
			b.BucketTS = ts.Time
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// TopSites returns sites ranked by requests in a time range.
func (r *TrafficMetricsRepository) TopSites(ctx context.Context, from, to time.Time, limit int) ([]TrafficBucket, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT site, SUM(requests), SUM(bytes), SUM(s2xx), SUM(s3xx), SUM(s4xx), SUM(s5xx)
		FROM traffic_metrics
		WHERE bucket_ts >= ? AND bucket_ts < ?
		GROUP BY site
		ORDER BY SUM(requests) DESC
		LIMIT ?
	`, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("top sites: %w", err)
	}
	defer rows.Close()

	var out []TrafficBucket
	for rows.Next() {
		var b TrafficBucket
		if err := rows.Scan(&b.Site, &b.Requests, &b.Bytes, &b.S2xx, &b.S3xx, &b.S4xx, &b.S5xx); err != nil {
			return nil, fmt.Errorf("scan top site: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// StatusCodeTotals aggregates status code buckets in a range.
func (r *TrafficMetricsRepository) StatusCodeTotals(ctx context.Context, from, to time.Time, site string) (s2xx, s3xx, s4xx, s5xx int, err error) {
	query := `
		SELECT COALESCE(SUM(s2xx),0), COALESCE(SUM(s3xx),0), COALESCE(SUM(s4xx),0), COALESCE(SUM(s5xx),0)
		FROM traffic_metrics
		WHERE bucket_ts >= ? AND bucket_ts < ?`
	args := []interface{}{from, to}
	if site != "" {
		query += ` AND site = ?`
		args = append(args, site)
	}
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&s2xx, &s3xx, &s4xx, &s5xx); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("status code totals: %w", err)
	}
	return s2xx, s3xx, s4xx, s5xx, nil
}

// SummaryTotals aggregates requests and bytes in a range.
func (r *TrafficMetricsRepository) SummaryTotals(ctx context.Context, from, to time.Time) (requests, bytes int, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(requests),0), COALESCE(SUM(bytes),0)
		FROM traffic_metrics
		WHERE bucket_ts >= ? AND bucket_ts < ?
	`, from, to).Scan(&requests, &bytes)
	if err != nil {
		return 0, 0, fmt.Errorf("summary totals: %w", err)
	}
	return requests, bytes, nil
}

// PurgeOlderThan deletes buckets before cutoff.
func (r *TrafficMetricsRepository) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM traffic_metrics WHERE bucket_ts < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge traffic metrics: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
