package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	RunEveryMinute = "min"
	RunEveryHour   = "hour"
	RunEveryDay    = "day"
	RunEveryMonth  = "month"
)

// CronJob is a scheduled shell command.
type CronJob struct {
	ID         int64
	Name       string
	Payload    string
	RunEvery   string
	ExecutedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CronJobRepository persists cron job records.
type CronJobRepository struct {
	db *sql.DB
}

// NewCronJobRepository returns a cron job repository backed by db.
func NewCronJobRepository(db *sql.DB) *CronJobRepository {
	return &CronJobRepository{db: db}
}

// List returns all cron jobs ordered by id.
func (r *CronJobRepository) List(ctx context.Context) ([]CronJob, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, payload, run_every, executed_at, created_at, updated_at
		FROM cronjobs ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list cronjobs: %w", err)
	}
	defer rows.Close()

	var jobs []CronJob
	for rows.Next() {
		job, err := scanCronJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// FindByID loads a cron job by primary key.
func (r *CronJobRepository) FindByID(ctx context.Context, id int64) (CronJob, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, payload, run_every, executed_at, created_at, updated_at
		FROM cronjobs WHERE id = ?
	`, id)
	job, err := scanCronJob(row)
	if err != nil {
		return CronJob{}, fmt.Errorf("find cronjob by id: %w", err)
	}
	return job, nil
}

// Create inserts a cron job and returns the stored row.
func (r *CronJobRepository) Create(ctx context.Context, job CronJob) (CronJob, error) {
	now := time.Now().UTC()
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO cronjobs (name, payload, run_every, executed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, job.Name, job.Payload, job.RunEvery, nullTime(job.ExecutedAt), now, now)
	if err != nil {
		return CronJob{}, fmt.Errorf("insert cronjob: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return CronJob{}, fmt.Errorf("last insert id: %w", err)
	}
	return r.FindByID(ctx, id)
}

// Update replaces mutable cron job fields.
func (r *CronJobRepository) Update(ctx context.Context, job CronJob) (CronJob, error) {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE cronjobs
		SET name = ?, payload = ?, run_every = ?, executed_at = ?, updated_at = ?
		WHERE id = ?
	`, job.Name, job.Payload, job.RunEvery, nullTime(job.ExecutedAt), now, job.ID)
	if err != nil {
		return CronJob{}, fmt.Errorf("update cronjob: %w", err)
	}
	return r.FindByID(ctx, job.ID)
}

// Delete removes a cron job by id.
func (r *CronJobRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM cronjobs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete cronjob: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// TouchExecutedAt updates executed_at to now.
func (r *CronJobRepository) TouchExecutedAt(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE cronjobs SET executed_at = ?, updated_at = ? WHERE id = ?
	`, now, now, id)
	if err != nil {
		return fmt.Errorf("touch executed_at: %w", err)
	}
	return nil
}

type cronScanner interface {
	Scan(dest ...any) error
}

func scanCronJob(row cronScanner) (CronJob, error) {
	var job CronJob
	var executedAt, createdAt, updatedAt sql.NullTime
	if err := row.Scan(&job.ID, &job.Name, &job.Payload, &job.RunEvery,
		&executedAt, &createdAt, &updatedAt); err != nil {
		return CronJob{}, err
	}
	if executedAt.Valid {
		t := executedAt.Time
		job.ExecutedAt = &t
	}
	if createdAt.Valid {
		job.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		job.UpdatedAt = updatedAt.Time
	}
	return job, nil
}
