package cron_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/service/cron"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/jahrulnr/gosite/pkg/apperror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCron(t *testing.T) (*cron.Service, *sqlite.CronJobRepository, *sqlite.JobRepository, *job.Worker) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	cronRepo := sqlite.NewCronJobRepository(db)
	jobRepo := sqlite.NewJobRepository(db)
	cmd := testutil.NewMockCommander()
	cmd.Stdout = "done"
	worker := job.NewWorker(jobRepo, cmd, 8)
	worker.Start(context.Background(), 1)
	t.Cleanup(worker.Stop)
	return cron.NewService(cronRepo, jobRepo, worker), cronRepo, jobRepo, worker
}

func TestCron_CreateAndList(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	created, err := svc.Create(context.Background(), cron.CreateInput{
		Name: "test", Payload: "echo hi", RunEvery: sqlite.RunEveryDay,
	})
	require.NoError(t, err)
	jobs, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	assert.Equal(t, created.ID, jobs[0].ID)
}

func TestCron_InvalidRunEvery(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	_, err := svc.Create(context.Background(), cron.CreateInput{
		Name: "bad", Payload: "echo", RunEvery: "weekly",
	})
	require.Error(t, err)
	assert.Equal(t, apperror.CodeCronInvalid, apperror.From(err).Code)
}

func TestCron_Update(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	created, err := svc.Create(context.Background(), cron.CreateInput{
		Name: "old", Payload: "echo old", RunEvery: sqlite.RunEveryHour,
	})
	require.NoError(t, err)
	updated, err := svc.Update(context.Background(), created.ID, cron.CreateInput{
		Name: "new", Payload: "echo new", RunEvery: sqlite.RunEveryDay,
	})
	require.NoError(t, err)
	assert.Equal(t, "new", updated.Name)
}

func TestCron_Delete(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	created, err := svc.Create(context.Background(), cron.CreateInput{
		Name: "rm", Payload: "echo", RunEvery: sqlite.RunEveryMinute,
	})
	require.NoError(t, err)
	require.NoError(t, svc.Delete(context.Background(), created.ID))
	jobs, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, jobs)
}

func TestCron_RunManualCreatesJob(t *testing.T) {
	svc, _, jobRepo, _ := setupCron(t)
	created, err := svc.Create(context.Background(), cron.CreateInput{
		Name: "run", Payload: "echo manual", RunEvery: sqlite.RunEveryDay,
	})
	require.NoError(t, err)
	run, err := svc.RunManual(context.Background(), created.ID)
	require.NoError(t, err)
	assert.NotZero(t, run.ID)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stored, findErr := jobRepo.FindByID(context.Background(), run.ID)
		require.NoError(t, findErr)
		if stored.Status == sqlite.JobStatusOK {
			assert.Contains(t, stored.Output, "echo manual")
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not complete")
}

func TestCron_MonthRollover(t *testing.T) {
	jan31 := time.Date(2026, time.January, 31, 23, 59, 0, 0, time.UTC)
	feb1 := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	assert.False(t, cron.ShouldRun(jan31, jan31, sqlite.RunEveryMonth))
	assert.True(t, cron.ShouldRun(jan31, feb1, sqlite.RunEveryMonth))

	dec31 := time.Date(2025, time.December, 31, 23, 59, 0, 0, time.UTC)
	jan1 := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	assert.True(t, cron.ShouldRun(dec31, jan1, sqlite.RunEveryMonth))
}

func TestCron_ShouldRunMinute(t *testing.T) {
	a := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	b := time.Date(2026, 6, 14, 10, 1, 0, 0, time.UTC)
	assert.True(t, cron.ShouldRun(a, b, sqlite.RunEveryMinute))
	assert.False(t, cron.ShouldRun(a, a, sqlite.RunEveryMinute))
}

func TestCron_ShouldRunHour(t *testing.T) {
	a := time.Date(2026, 6, 14, 10, 30, 0, 0, time.UTC)
	b := time.Date(2026, 6, 14, 11, 30, 0, 0, time.UTC)
	assert.True(t, cron.ShouldRun(a, b, sqlite.RunEveryHour))
}

func TestCron_ShouldRunDay(t *testing.T) {
	a := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	b := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	assert.True(t, cron.ShouldRun(a, b, sqlite.RunEveryDay))
	assert.False(t, cron.ShouldRun(a, a, sqlite.RunEveryDay))
}

func TestCron_GetNotFound(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	_, err := svc.Get(context.Background(), 999)
	require.Error(t, err)
}

func TestCron_CreateMissingFields(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	_, err := svc.Create(context.Background(), cron.CreateInput{RunEvery: sqlite.RunEveryDay})
	require.Error(t, err)
}

func TestCron_DeleteNotFound(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	err := svc.Delete(context.Background(), 404)
	require.Error(t, err)
}

func TestCron_GetJobRun(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	created, err := svc.Create(context.Background(), cron.CreateInput{
		Name: "stream", Payload: "echo stream", RunEvery: sqlite.RunEveryDay,
	})
	require.NoError(t, err)

	run, err := svc.RunManual(context.Background(), created.ID)
	require.NoError(t, err)

	got, err := svc.GetJobRun(context.Background(), run.ID)
	require.NoError(t, err)
	assert.Equal(t, run.ID, got.ID)
}

func TestCron_GetJobRunNotFound(t *testing.T) {
	svc, _, _, _ := setupCron(t)
	_, err := svc.GetJobRun(context.Background(), 99999)
	require.Error(t, err)
}
