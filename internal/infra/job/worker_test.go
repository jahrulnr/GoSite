package job_test

import (
	"bytes"
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jahrulnr/gosite/internal/infra/job"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupWorker(t *testing.T) (*job.Worker, *sqlite.JobRepository, *testutil.MockCommander) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "gosite.db")
	db, err := sqlite.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, sqlite.Migrate(db, filepath.Clean(filepath.Join("..", "..", "..", "migrations"))))
	jobs := sqlite.NewJobRepository(db)
	cmd := testutil.NewMockCommander()
	cmd.Stdout = "worker-output"
	w := job.NewWorker(jobs, cmd, 4)
	w.Start(context.Background(), 1)
	t.Cleanup(w.Stop)
	return w, jobs, cmd
}

func TestJob_SSEStreamsOutput(t *testing.T) {
	w, jobs, _ := setupWorker(t)
	ctx := context.Background()
	run, err := jobs.Create(ctx, sqlite.JobRun{
		JobType: "cron",
		Name:    "stream",
		Status:  sqlite.JobStatusPending,
		Output:  "echo streamed",
	})
	require.NoError(t, err)
	w.Enqueue(run.ID)

	rec := httptest.NewRecorder()
	done := make(chan error, 1)
	go func() {
		done <- w.StreamSSE(ctx, rec, run.ID)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for SSE")
	}

	body := rec.Body.String()
	assert.Contains(t, body, "echo streamed")
	assert.Contains(t, body, "worker-output")
	assert.Contains(t, body, "event: done")
}

func TestJob_ProcessCompletesOK(t *testing.T) {
	w, jobs, _ := setupWorker(t)
	ctx := context.Background()
	run, err := jobs.Create(ctx, sqlite.JobRun{
		JobType: "cron", Name: "ok", Status: sqlite.JobStatusPending, Output: "echo ok",
	})
	require.NoError(t, err)
	w.Enqueue(run.ID)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stored, findErr := jobs.FindByID(ctx, run.ID)
		require.NoError(t, findErr)
		if stored.Status == sqlite.JobStatusOK {
			assert.Contains(t, stored.Output, "worker-output")
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job not completed")
}

func TestJob_ProcessFailsOnEmptyCommand(t *testing.T) {
	w, jobs, _ := setupWorker(t)
	ctx := context.Background()
	run, err := jobs.Create(ctx, sqlite.JobRun{
		JobType: "cron", Name: "empty", Status: sqlite.JobStatusPending, Output: "   ",
	})
	require.NoError(t, err)
	w.Enqueue(run.ID)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stored, findErr := jobs.FindByID(ctx, run.ID)
		require.NoError(t, findErr)
		if stored.Status == sqlite.JobStatusFailed {
			assert.Contains(t, stored.Error, "empty command")
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job not failed")
}

func TestJob_EnqueueProcessesMultiple(t *testing.T) {
	w, jobs, _ := setupWorker(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		run, err := jobs.Create(ctx, sqlite.JobRun{
			JobType: "cron", Name: "multi", Status: sqlite.JobStatusPending, Output: "echo x",
		})
		require.NoError(t, err)
		w.Enqueue(run.ID)
	}
	time.Sleep(300 * time.Millisecond)
	list, err := jobs.FindByID(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, sqlite.JobStatusOK, list.Status)
}

func TestJob_StreamNotFound(t *testing.T) {
	w, _, _ := setupWorker(t)
	rec := httptest.NewRecorder()
	err := w.StreamSSE(context.Background(), rec, 9999)
	require.Error(t, err)
}

func TestJob_StreamIncludesPartialOutput(t *testing.T) {
	w, jobs, _ := setupWorker(t)
	ctx := context.Background()
	run, err := jobs.Create(ctx, sqlite.JobRun{
		JobType: "cron", Name: "partial", Status: sqlite.JobStatusRunning, Output: "partial chunk",
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = jobs.Complete(ctx, run.ID, sqlite.JobStatusOK, "partial chunk done", "")
	}()
	err = w.StreamSSE(ctx, rec, run.ID)
	require.NoError(t, err)
	assert.True(t, strings.Contains(rec.Body.String(), "partial"))
}

func TestJob_ProcessUsesShell(t *testing.T) {
	w, jobs, cmd := setupWorker(t)
	ctx := context.Background()
	run, err := jobs.Create(ctx, sqlite.JobRun{
		JobType: "cron", Name: "shell", Status: sqlite.JobStatusPending, Output: "echo shell",
	})
	require.NoError(t, err)
	w.Enqueue(run.ID)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		calls := cmd.SnapshotCalls()
		if len(calls) > 0 {
			assert.Equal(t, "sh", calls[0].Name)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("command not executed")
}

func TestJob_NewWorkerDefaultBuffer(t *testing.T) {
	jobs := sqlite.NewJobRepository(nil)
	w := job.NewWorker(jobs, testutil.NewMockCommander(), 0)
	require.NotNil(t, w)
}

func TestJob_StreamDoneOnFailure(t *testing.T) {
	w, jobs, _ := setupWorker(t)
	ctx := context.Background()
	run, err := jobs.Create(ctx, sqlite.JobRun{
		JobType: "cron", Name: "fail", Status: sqlite.JobStatusFailed, Output: "x", Error: "boom",
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	err = w.StreamSSE(ctx, rec, run.ID)
	require.Error(t, err)
	assert.Contains(t, rec.Body.String(), "event: done")
}

func TestJob_StreamFlushesIncremental(t *testing.T) {
	w, jobs, _ := setupWorker(t)
	ctx := context.Background()
	run, err := jobs.Create(ctx, sqlite.JobRun{
		JobType: "cron", Name: "inc", Status: sqlite.JobStatusRunning, Output: "line1\n",
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = jobs.Complete(ctx, run.ID, sqlite.JobStatusOK, "line1\nline2\n", "")
	}()
	require.NoError(t, w.StreamSSE(ctx, rec, run.ID))
	assert.True(t, bytes.Contains(rec.Body.Bytes(), []byte("line2")))
}

func TestJob_MarkRunningSetsStartedAt(t *testing.T) {
	_, jobs, _ := setupWorker(t)
	ctx := context.Background()
	run, err := jobs.Create(ctx, sqlite.JobRun{JobType: "cron", Name: "t", Status: sqlite.JobStatusPending, Output: "echo"})
	require.NoError(t, err)
	require.NoError(t, jobs.MarkRunning(ctx, run.ID))
	stored, err := jobs.FindByID(ctx, run.ID)
	require.NoError(t, err)
	assert.Equal(t, sqlite.JobStatusRunning, stored.Status)
	assert.NotNil(t, stored.StartedAt)
}
