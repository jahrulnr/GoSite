package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// NginxStatusSample is a stub_status snapshot row.
type NginxStatusSample struct {
	ID       int64
	SampleTS time.Time
	Active   int
	Accepts  int64
	Handled  int64
	Requests int64
	Reading  int
	Writing  int
	Waiting  int
}

// NginxStatusRepository persists nginx stub_status samples.
type NginxStatusRepository struct {
	db *sql.DB
}

// NewNginxStatusRepository returns a nginx status repository backed by db.
func NewNginxStatusRepository(db *sql.DB) *NginxStatusRepository {
	return &NginxStatusRepository{db: db}
}

// InsertSample stores a new stub_status sample.
func (r *NginxStatusRepository) InsertSample(ctx context.Context, s NginxStatusSample) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO nginx_status_samples
			(sample_ts, active, accepts, handled, requests, reading, writing, waiting)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, s.SampleTS, s.Active, s.Accepts, s.Handled, s.Requests, s.Reading, s.Writing, s.Waiting)
	if err != nil {
		return fmt.Errorf("insert nginx status sample: %w", err)
	}
	return nil
}

// LatestSample returns the most recent sample, if any.
func (r *NginxStatusRepository) LatestSample(ctx context.Context) (NginxStatusSample, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, sample_ts, active, accepts, handled, requests, reading, writing, waiting
		FROM nginx_status_samples
		ORDER BY sample_ts DESC, id DESC
		LIMIT 1
	`)
	var s NginxStatusSample
	var ts sql.NullTime
	if err := row.Scan(&s.ID, &ts, &s.Active, &s.Accepts, &s.Handled, &s.Requests, &s.Reading, &s.Writing, &s.Waiting); err != nil {
		if err == sql.ErrNoRows {
			return NginxStatusSample{}, false, nil
		}
		return NginxStatusSample{}, false, fmt.Errorf("scan latest nginx status: %w", err)
	}
	if ts.Valid {
		s.SampleTS = ts.Time
	}
	return s, true, nil
}

// PreviousSample returns the sample immediately before the given timestamp.
func (r *NginxStatusRepository) PreviousSample(ctx context.Context, before time.Time) (NginxStatusSample, bool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, sample_ts, active, accepts, handled, requests, reading, writing, waiting
		FROM nginx_status_samples
		WHERE sample_ts < ?
		ORDER BY sample_ts DESC, id DESC
		LIMIT 1
	`, before)
	var s NginxStatusSample
	var ts sql.NullTime
	if err := row.Scan(&s.ID, &ts, &s.Active, &s.Accepts, &s.Handled, &s.Requests, &s.Reading, &s.Writing, &s.Waiting); err != nil {
		if err == sql.ErrNoRows {
			return NginxStatusSample{}, false, nil
		}
		return NginxStatusSample{}, false, fmt.Errorf("scan previous nginx status: %w", err)
	}
	if ts.Valid {
		s.SampleTS = ts.Time
	}
	return s, true, nil
}

// ListSamples returns samples in [from, to).
func (r *NginxStatusRepository) ListSamples(ctx context.Context, from, to time.Time) ([]NginxStatusSample, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sample_ts, active, accepts, handled, requests, reading, writing, waiting
		FROM nginx_status_samples
		WHERE sample_ts >= ? AND sample_ts < ?
		ORDER BY sample_ts ASC, id ASC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("list nginx status samples: %w", err)
	}
	defer rows.Close()

	var out []NginxStatusSample
	for rows.Next() {
		var s NginxStatusSample
		var ts sql.NullTime
		if err := rows.Scan(&s.ID, &ts, &s.Active, &s.Accepts, &s.Handled, &s.Requests, &s.Reading, &s.Writing, &s.Waiting); err != nil {
			return nil, fmt.Errorf("scan nginx status sample: %w", err)
		}
		if ts.Valid {
			s.SampleTS = ts.Time
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// PurgeOlderThan deletes samples before cutoff.
func (r *NginxStatusRepository) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM nginx_status_samples WHERE sample_ts < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("purge nginx status samples: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
