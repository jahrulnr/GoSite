package cron

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Service manages cron job CRUD and manual runs.
type Service struct {
	repo   *sqlite.CronJobRepository
	jobs   *sqlite.JobRepository
	worker *job.Worker
	hooks  contracts.HookBus
}

// Option configures cron service dependencies.
type Option func(*Service)

// WithHookBus dispatches cron lifecycle events to plugins.
func WithHookBus(hooks contracts.HookBus) Option {
	return func(s *Service) {
		if hooks != nil {
			s.hooks = hooks
		}
	}
}

// NewService returns a cron service.
func NewService(repo *sqlite.CronJobRepository, jobs *sqlite.JobRepository, worker *job.Worker, opts ...Option) *Service {
	svc := &Service{repo: repo, jobs: jobs, worker: worker, hooks: contracts.NoopHookBus{}}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// CreateInput holds cron job creation fields.
type CreateInput struct {
	Name     string
	Payload  string
	RunEvery string
}

// List returns all cron jobs.
func (s *Service) List(ctx context.Context) ([]sqlite.CronJob, error) {
	jobs, err := s.repo.List(ctx)
	if err != nil {
		return nil, apperror.Wrap(apperror.CodeDatabase, "list cronjobs", err)
	}
	return jobs, nil
}

// Get returns one cron job.
func (s *Service) Get(ctx context.Context, id int64) (sqlite.CronJob, error) {
	job, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return sqlite.CronJob{}, apperror.Wrap(apperror.CodeNotFound, "cronjob not found", err)
	}
	return job, nil
}

// Create inserts a cron job.
func (s *Service) Create(ctx context.Context, in CreateInput) (sqlite.CronJob, error) {
	if err := validateRunEvery(in.RunEvery); err != nil {
		return sqlite.CronJob{}, err
	}
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Payload) == "" {
		return sqlite.CronJob{}, apperror.New(apperror.CodeInvalidInput, "name and payload required")
	}
	job, err := s.repo.Create(ctx, sqlite.CronJob{
		Name:     in.Name,
		Payload:  in.Payload,
		RunEvery: strings.ToLower(strings.TrimSpace(in.RunEvery)),
	})
	if err != nil {
		return sqlite.CronJob{}, apperror.Wrap(apperror.CodeDatabase, "create cronjob", err)
	}
	return job, nil
}

// Update replaces cron job fields.
func (s *Service) Update(ctx context.Context, id int64, in CreateInput) (sqlite.CronJob, error) {
	if err := validateRunEvery(in.RunEvery); err != nil {
		return sqlite.CronJob{}, err
	}
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return sqlite.CronJob{}, apperror.Wrap(apperror.CodeNotFound, "cronjob not found", err)
	}
	existing.Name = in.Name
	existing.Payload = in.Payload
	existing.RunEvery = strings.ToLower(strings.TrimSpace(in.RunEvery))
	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return sqlite.CronJob{}, apperror.Wrap(apperror.CodeDatabase, "update cronjob", err)
	}
	return updated, nil
}

// Delete removes a cron job.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if err == sql.ErrNoRows {
			return apperror.New(apperror.CodeNotFound, "cronjob not found")
		}
		return apperror.Wrap(apperror.CodeDatabase, "delete cronjob", err)
	}
	return nil
}

// RunManual enqueues a manual cron execution and returns the job run id.
func (s *Service) RunManual(ctx context.Context, cronID int64) (sqlite.JobRun, error) {
	cronJob, err := s.repo.FindByID(ctx, cronID)
	if err != nil {
		return sqlite.JobRun{}, apperror.Wrap(apperror.CodeNotFound, "cronjob not found", err)
	}
	if _, err := s.hooks.Dispatch(ctx, "cron.before_trigger", map[string]any{
		"id":        cronJob.ID,
		"name":      cronJob.Name,
		"run_every": cronJob.RunEvery,
	}); err != nil {
		return sqlite.JobRun{}, err
	}
	run, err := s.jobs.Create(ctx, sqlite.JobRun{
		JobType: "cron",
		Name:    cronJob.Name,
		Status:  sqlite.JobStatusPending,
		Output:  cronJob.Payload,
	})
	if err != nil {
		return sqlite.JobRun{}, apperror.Wrap(apperror.CodeDatabase, "create job run", err)
	}
	s.worker.Enqueue(run.ID)
	if err := s.repo.TouchExecutedAt(ctx, cronID); err != nil {
		return sqlite.JobRun{}, apperror.Wrap(apperror.CodeDatabase, "touch executed_at", err)
	}
	return run, nil
}

// GetJobRun returns a job run for streaming.
func (s *Service) GetJobRun(ctx context.Context, jobID int64) (sqlite.JobRun, error) {
	run, err := s.jobs.FindByID(ctx, jobID)
	if err != nil {
		return sqlite.JobRun{}, apperror.Wrap(apperror.CodeNotFound, "job not found", err)
	}
	return run, nil
}

// ShouldRun reports whether a cron interval boundary was crossed.
func ShouldRun(prev, now time.Time, runEvery string) bool {
	switch strings.ToLower(strings.TrimSpace(runEvery)) {
	case sqlite.RunEveryMinute:
		return prev.Minute() != now.Minute() || !sameClockDay(prev, now)
	case sqlite.RunEveryHour:
		return prev.Hour() != now.Hour() || !sameClockDay(prev, now)
	case sqlite.RunEveryDay:
		return !sameClockDay(prev, now)
	case sqlite.RunEveryMonth:
		return prev.Month() != now.Month() || prev.Year() != now.Year()
	default:
		return false
	}
}

func sameClockDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func validateRunEvery(runEvery string) error {
	switch strings.ToLower(strings.TrimSpace(runEvery)) {
	case sqlite.RunEveryMinute, sqlite.RunEveryHour, sqlite.RunEveryDay, sqlite.RunEveryMonth:
		return nil
	default:
		return apperror.New(apperror.CodeCronInvalid, fmt.Sprintf("invalid run_every: %s", runEvery))
	}
}
