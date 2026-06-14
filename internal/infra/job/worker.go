package job

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jahrulnr/gosite/internal/contracts"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
	"github.com/jahrulnr/gosite/pkg/apperror"
)

// Worker executes queued job runs and exposes SSE streaming.
type Worker struct {
	jobs   *sqlite.JobRepository
	cmd    contracts.CommandRunner
	queue  chan int64
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewWorker returns a job worker.
func NewWorker(jobs *sqlite.JobRepository, cmd contracts.CommandRunner, buffer int) *Worker {
	if buffer <= 0 {
		buffer = 32
	}
	return &Worker{
		jobs:   jobs,
		cmd:    cmd,
		queue:  make(chan int64, buffer),
		stopCh: make(chan struct{}),
	}
}

// Start launches background workers.
func (w *Worker) Start(ctx context.Context, workers int) {
	if workers <= 0 {
		workers = 1
	}
	for i := 0; i < workers; i++ {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.loop(ctx)
		}()
	}
}

// Stop closes the worker loop and waits for in-flight jobs.
func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

// Enqueue schedules a job run id.
func (w *Worker) Enqueue(id int64) {
	select {
	case w.queue <- id:
	default:
		go func() { w.queue <- id }()
	}
}

func (w *Worker) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case id := <-w.queue:
			w.process(ctx, id)
		}
	}
}

func (w *Worker) process(ctx context.Context, id int64) {
	job, err := w.jobs.FindByID(ctx, id)
	if err != nil {
		return
	}

	command := strings.TrimSpace(job.Output)
	if command == "" {
		_ = w.jobs.Complete(ctx, id, sqlite.JobStatusFailed, "", "empty command")
		return
	}

	if err := w.jobs.MarkRunning(ctx, id); err != nil {
		return
	}

	output := fmt.Sprintf("$ %s\n", command)
	_ = w.jobs.AppendOutput(ctx, id, output)

	var res contracts.CommandResult
	var runErr error
	if streaming, ok := w.cmd.(contracts.StreamingCommandRunner); ok {
		res, runErr = streaming.RunStreaming(ctx, "sh", []string{"-c", command}, func(stream, chunk string) {
			if stream == "stderr" {
				chunk = "[stderr] " + chunk
			}
			_ = w.jobs.AppendOutput(ctx, id, chunk)
		})
		stored, err := w.jobs.FindByID(ctx, id)
		if err == nil {
			output = stored.Output
		}
	} else {
		res, runErr = w.cmd.Run(ctx, "sh", "-c", command)
		if strings.TrimSpace(res.Stdout) != "" {
			output += strings.TrimSpace(res.Stdout)
		}
		if res.Stderr != "" {
			if !strings.HasSuffix(output, "\n") && output != "" {
				output += "\n"
			}
			output += strings.TrimSpace(res.Stderr)
		}
	}
	if runErr != nil || res.ExitCode != 0 {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		_ = w.jobs.Complete(ctx, id, sqlite.JobStatusFailed, output, msg)
		return
	}
	_ = w.jobs.Complete(ctx, id, sqlite.JobStatusOK, output, "")
}

// StreamSSE writes job output as server-sent events until completion.
func (w *Worker) StreamSSE(ctx context.Context, rw http.ResponseWriter, jobID int64) error {
	flusher, ok := rw.(http.Flusher)
	if !ok {
		return apperror.New(apperror.CodeInternal, "streaming unsupported")
	}

	rw.Header().Set("Content-Type", "text/event-stream")
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Connection", "keep-alive")
	rw.Header().Set("X-Accel-Buffering", "no")

	lastLen := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		job, err := w.jobs.FindByID(ctx, jobID)
		if err != nil {
			return apperror.Wrap(apperror.CodeNotFound, "job not found", err)
		}

		if len(job.Output) > lastLen {
			chunk := job.Output[lastLen:]
			writeSSEData(rw, chunk)
			flusher.Flush()
			lastLen = len(job.Output)
		}

		switch job.Status {
		case sqlite.JobStatusOK:
			fmt.Fprintf(rw, "event: done\ndata: ok\n\n")
			flusher.Flush()
			return nil
		case sqlite.JobStatusFailed, sqlite.JobStatusCancelled:
			msg := job.Error
			if msg == "" {
				msg = "failed"
			}
			fmt.Fprintf(rw, "event: done\ndata: %s\n\n", msg)
			flusher.Flush()
			return apperror.New(apperror.CodeJobFailed, msg)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func writeSSEData(w http.ResponseWriter, chunk string) {
	if chunk == "" {
		return
	}
	chunk = strings.ReplaceAll(chunk, "\r\n", "\n")
	chunk = strings.ReplaceAll(chunk, "\r", "\n")
	for _, line := range strings.SplitAfter(chunk, "\n") {
		line = strings.TrimSuffix(line, "\n")
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
