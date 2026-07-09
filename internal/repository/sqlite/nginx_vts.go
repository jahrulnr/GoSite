package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// VTSServerSample is a per-server_name VTS snapshot row.
type VTSServerSample struct {
	ID          int64
	SampleTS    time.Time
	ServerName  string
	Requests    int
	InBytes     int
	OutBytes    int
	RequestMsec float64
}

// VTSUpstreamSample is a per-upstream peer VTS snapshot row.
type VTSUpstreamSample struct {
	ID           int64
	SampleTS     time.Time
	UpstreamName string
	ServerAddr   string
	Requests     int
	InBytes      int
	OutBytes     int
	ResponseMsec float64
	Down         bool
}

// NginxVTSRepository persists VTS metric snapshots.
type NginxVTSRepository struct {
	db *sql.DB
}

// NewNginxVTSRepository returns a VTS metrics repository backed by db.
func NewNginxVTSRepository(db *sql.DB) *NginxVTSRepository {
	return &NginxVTSRepository{db: db}
}

// InsertServerSample stores a server zone snapshot.
func (r *NginxVTSRepository) InsertServerSample(ctx context.Context, s VTSServerSample) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO nginx_vts_server_samples
			(sample_ts, server_name, requests, in_bytes, out_bytes, request_msec)
		VALUES (?, ?, ?, ?, ?, ?)
	`, s.SampleTS, s.ServerName, s.Requests, s.InBytes, s.OutBytes, s.RequestMsec)
	if err != nil {
		return fmt.Errorf("insert vts server sample: %w", err)
	}
	return nil
}

// InsertUpstreamSample stores an upstream peer snapshot.
func (r *NginxVTSRepository) InsertUpstreamSample(ctx context.Context, s VTSUpstreamSample) error {
	down := 0
	if s.Down {
		down = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO nginx_vts_upstream_samples
			(sample_ts, upstream_name, server_addr, requests, in_bytes, out_bytes, response_msec, down)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, s.SampleTS, s.UpstreamName, s.ServerAddr, s.Requests, s.InBytes, s.OutBytes, s.ResponseMsec, down)
	if err != nil {
		return fmt.Errorf("insert vts upstream sample: %w", err)
	}
	return nil
}

// LatestSampleTS returns the newest VTS sample timestamp, if any.
func (r *NginxVTSRepository) LatestSampleTS(ctx context.Context) (time.Time, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT sample_ts FROM nginx_vts_server_samples
		UNION ALL
		SELECT sample_ts FROM nginx_vts_upstream_samples
		ORDER BY sample_ts DESC
		LIMIT 1
	`)
	var ts sql.NullTime
	if err := row.Scan(&ts); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("scan latest vts sample ts: %w", err)
	}
	if !ts.Valid {
		return time.Time{}, false, nil
	}
	return ts.Time, true, nil
}

// TopServersAtLatest returns server rows from the newest snapshot batch.
func (r *NginxVTSRepository) TopServersAtLatest(ctx context.Context, limit int) ([]VTSServerSample, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sample_ts, server_name, requests, in_bytes, out_bytes, request_msec
		FROM nginx_vts_server_samples
		WHERE sample_ts = (SELECT MAX(sample_ts) FROM nginx_vts_server_samples)
		ORDER BY requests DESC, server_name ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("top vts servers: %w", err)
	}
	defer rows.Close()
	return scanServerSamples(rows)
}

// TopUpstreamsAtLatest returns upstream peer rows from the newest snapshot batch.
func (r *NginxVTSRepository) TopUpstreamsAtLatest(ctx context.Context, limit int) ([]VTSUpstreamSample, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sample_ts, upstream_name, server_addr, requests, in_bytes, out_bytes, response_msec, down
		FROM nginx_vts_upstream_samples
		WHERE sample_ts = (SELECT MAX(sample_ts) FROM nginx_vts_upstream_samples)
		ORDER BY requests DESC, upstream_name ASC, server_addr ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("top vts upstreams: %w", err)
	}
	defer rows.Close()
	return scanUpstreamSamples(rows)
}

// PurgeOlderThan deletes VTS samples before cutoff.
func (r *NginxVTSRepository) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for _, table := range []string{"nginx_vts_server_samples", "nginx_vts_upstream_samples"} {
		res, err := r.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE sample_ts < ?", table), cutoff)
		if err != nil {
			return total, fmt.Errorf("purge %s: %w", table, err)
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

func scanServerSamples(rows *sql.Rows) ([]VTSServerSample, error) {
	var out []VTSServerSample
	for rows.Next() {
		var s VTSServerSample
		var ts sql.NullTime
		if err := rows.Scan(&s.ID, &ts, &s.ServerName, &s.Requests, &s.InBytes, &s.OutBytes, &s.RequestMsec); err != nil {
			return nil, fmt.Errorf("scan vts server sample: %w", err)
		}
		if ts.Valid {
			s.SampleTS = ts.Time
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanUpstreamSamples(rows *sql.Rows) ([]VTSUpstreamSample, error) {
	var out []VTSUpstreamSample
	for rows.Next() {
		var s VTSUpstreamSample
		var ts sql.NullTime
		var down int
		if err := rows.Scan(&s.ID, &ts, &s.UpstreamName, &s.ServerAddr, &s.Requests, &s.InBytes, &s.OutBytes, &s.ResponseMsec, &down); err != nil {
			return nil, fmt.Errorf("scan vts upstream sample: %w", err)
		}
		if ts.Valid {
			s.SampleTS = ts.Time
		}
		s.Down = down != 0
		out = append(out, s)
	}
	return out, rows.Err()
}
