package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// JobRun represents a background job execution record.
type JobRun struct {
	ID         int64
	JobType    string
	Name       string
	Status     string
	Output     string
	Error      string
	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

const (
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusOK        = "ok"
	JobStatusFailed    = "failed"
	JobStatusCancelled = "cancelled"
)

// JobRepository persists job run records.
type JobRepository struct {
	db *sql.DB
}

// NewJobRepository returns a job repository backed by db.
func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

// Create inserts a new job run and returns the stored row.
func (r *JobRepository) Create(ctx context.Context, job JobRun) (JobRun, error) {
	now := time.Now().UTC()
	if job.Status == "" {
		job.Status = JobStatusPending
	}
	res, err := r.db.ExecContext(ctx, `
		INSERT INTO job_runs (job_type, name, status, output, error, started_at, finished_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, job.JobType, job.Name, job.Status, job.Output, job.Error, nullTime(job.StartedAt), nullTime(job.FinishedAt), now, now)
	if err != nil {
		return JobRun{}, fmt.Errorf("insert job run: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return JobRun{}, fmt.Errorf("last insert id: %w", err)
	}
	return r.FindByID(ctx, id)
}

// FindByID loads a job run by primary key.
func (r *JobRepository) FindByID(ctx context.Context, id int64) (JobRun, error) {
	var job JobRun
	var startedAt, finishedAt sql.NullTime
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, job_type, name, status, output, error, started_at, finished_at, created_at, updated_at
		FROM job_runs WHERE id = ?
	`, id).Scan(&job.ID, &job.JobType, &job.Name, &job.Status, &job.Output, &job.Error,
		&startedAt, &finishedAt, &createdAt, &updatedAt)
	if err != nil {
		return JobRun{}, fmt.Errorf("find job by id: %w", err)
	}
	if startedAt.Valid {
		t := startedAt.Time
		job.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		job.FinishedAt = &t
	}
	if createdAt.Valid {
		job.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		job.UpdatedAt = updatedAt.Time
	}
	return job, nil
}

// UpdateStatus updates job status and optional output/error fields.
func (r *JobRepository) UpdateStatus(ctx context.Context, id int64, status, output, errMsg string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE job_runs SET status = ?, output = ?, error = ?, updated_at = ?
		WHERE id = ?
	`, status, output, errMsg, now, id)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

// MarkRunning sets status to running and records started_at.
func (r *JobRepository) MarkRunning(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE job_runs SET status = ?, started_at = ?, updated_at = ?
		WHERE id = ?
	`, JobStatusRunning, now, now, id)
	if err != nil {
		return fmt.Errorf("mark job running: %w", err)
	}
	return nil
}

// MarkRunningWithOutput sets status to running, records started_at, and
// replaces the output atomically.
func (r *JobRepository) MarkRunningWithOutput(ctx context.Context, id int64, output string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE job_runs SET status = ?, output = ?, started_at = ?, updated_at = ?
		WHERE id = ?
	`, JobStatusRunning, output, now, now, id)
	if err != nil {
		return fmt.Errorf("mark job running with output: %w", err)
	}
	return nil
}

// Complete marks a job finished with output and optional error message.
func (r *JobRepository) Complete(ctx context.Context, id int64, status, output, errMsg string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE job_runs SET status = ?, output = ?, error = ?, finished_at = ?, updated_at = ?
		WHERE id = ?
	`, status, output, errMsg, now, now, id)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	return nil
}

// CompleteStatus marks a job finished without overwriting output.
// Use this for streaming jobs where output was accumulated via AppendOutput.
func (r *JobRepository) CompleteStatus(ctx context.Context, id int64, status, errMsg string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE job_runs SET status = ?, error = ?, finished_at = ?, updated_at = ?
		WHERE id = ?
	`, status, errMsg, now, now, id)
	if err != nil {
		return fmt.Errorf("complete job status: %w", err)
	}
	return nil
}

// AppendOutput concatenates output text for streaming jobs.
func (r *JobRepository) AppendOutput(ctx context.Context, id int64, chunk string) error {
	now := time.Now().UTC()
	_, err := r.db.ExecContext(ctx, `
		UPDATE job_runs SET output = output || ?, updated_at = ?
		WHERE id = ?
	`, chunk, now, id)
	if err != nil {
		return fmt.Errorf("append job output: %w", err)
	}
	return nil
}

// JobFilter constrains job run queries.
type JobFilter struct {
	From   *time.Time
	To     *time.Time
	Wheres []string
	Args   []interface{}
	Limit  int
	Offset int
}

// List returns job runs matching filter ordered by created_at desc.
func (r *JobRepository) List(ctx context.Context, f JobFilter) ([]JobRun, error) {
	query := `
		SELECT id, job_type, name, status, output, error, started_at, finished_at, created_at, updated_at
		FROM job_runs WHERE 1=1`
	args := make([]interface{}, 0, len(f.Args)+4)
	if f.From != nil {
		query += ` AND created_at >= ?`
		args = append(args, *f.From)
	}
	if f.To != nil {
		query += ` AND created_at <= ?`
		args = append(args, *f.To)
	}
	for _, w := range f.Wheres {
		query += ` AND ` + w
	}
	args = append(args, f.Args...)
	query += ` ORDER BY created_at DESC`
	if f.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	if f.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, f.Offset)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list job runs: %w", err)
	}
	defer rows.Close()

	var out []JobRun
	for rows.Next() {
		job, err := scanJobRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

// Count returns matching job runs.
func (r *JobRepository) Count(ctx context.Context, f JobFilter) (int, error) {
	query := `SELECT COUNT(1) FROM job_runs WHERE 1=1`
	args := make([]interface{}, 0, len(f.Args)+2)
	if f.From != nil {
		query += ` AND created_at >= ?`
		args = append(args, *f.From)
	}
	if f.To != nil {
		query += ` AND created_at <= ?`
		args = append(args, *f.To)
	}
	for _, w := range f.Wheres {
		query += ` AND ` + w
	}
	args = append(args, f.Args...)
	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count job runs: %w", err)
	}
	return count, nil
}

// ListSince returns job runs with created_at > since, ordered by created_at asc.
func (r *JobRepository) ListSince(ctx context.Context, since time.Time) ([]JobRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, job_type, name, status, output, error, started_at, finished_at, created_at, updated_at
		FROM job_runs
		WHERE created_at > ?
		ORDER BY created_at ASC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("list job runs since: %w", err)
	}
	defer rows.Close()

	var out []JobRun
	for rows.Next() {
		job, err := scanJobRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

func scanJobRun(rows *sql.Rows) (JobRun, error) {
	var job JobRun
	var startedAt, finishedAt sql.NullTime
	var createdAt, updatedAt sql.NullTime
	if err := rows.Scan(&job.ID, &job.JobType, &job.Name, &job.Status, &job.Output, &job.Error,
		&startedAt, &finishedAt, &createdAt, &updatedAt); err != nil {
		return JobRun{}, fmt.Errorf("scan job run: %w", err)
	}
	if startedAt.Valid {
		t := startedAt.Time
		job.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		job.FinishedAt = &t
	}
	if createdAt.Valid {
		job.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		job.UpdatedAt = updatedAt.Time
	}
	return job, nil
}

func nullTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}
